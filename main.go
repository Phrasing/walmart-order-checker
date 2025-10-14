package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"walmart-order-checker/pkg/gmail"
	"walmart-order-checker/pkg/report"
	"walmart-order-checker/pkg/util"
)

const (
	defaultDays   = 10
	minDays       = 1
	walmartSender = "help@walmart.com"
)

type AccountConfig struct {
	Name            string // Display name (email or "[Root credentials]")
	Email           string // Empty for root until after auth
	CredentialsPath string
	TokenPath       string
	IsRoot          bool
}

func main() {
	daysFlag := flag.Int("days", defaultDays, "Number of days back to scan for emails")
	flag.Parse()

	// Discover all accounts
	accounts := discoverAccounts()

	if len(accounts) == 0 {
		log.Fatal("No Gmail accounts found. Please set up credentials.json in the root directory or in account folders.")
	}

	// Print discovered accounts
	fmt.Printf("Found %d Gmail account(s):\n", len(accounts))
	for i, acc := range accounts {
		fmt.Printf("  %d. %s\n", i+1, acc.Name)
	}
	fmt.Println()

	// Prompt for multi-account mode if 2+ accounts
	multiMode := false
	if len(accounts) >= 2 {
		multiMode = promptMultiAccountMode()
	}

	// Prompt for days
	maybePromptDays(daysFlag)
	days := *daysFlag

	if multiMode {
		// Check if all accounts have valid tokens
		allHaveTokens := true
		for _, acc := range accounts {
			if !hasValidToken(acc.TokenPath) {
				allHaveTokens = false
				break
			}
		}

		if allHaveTokens {
			fmt.Println("✓ All accounts authenticated - processing in parallel")
			processAccountsInParallel(accounts, days)
		} else {
			fmt.Println("⚠️  One or more accounts need authentication - processing sequentially")
			processAccountsSequentially(accounts, days)
		}
	} else {
		processSingleAccount(accounts[0], days)
	}
}

func maybePromptDays(days *int) bool {
	if len(os.Args) > 1 {
		return false
	}
	fmt.Print("Enter number of days to scan (default 10): ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		// If there's an error reading input (e.g., running without a console),
		// just use the default and continue.
		fmt.Printf("\nCould not read input, using default of %d days.\n", defaultDays)
		*days = defaultDays
		return true
	}
	input = strings.TrimSpace(input)
	if input == "" {
		*days = defaultDays
		return true
	}
	val, convErr := strconv.Atoi(input)
	if convErr != nil || val < minDays {
		fmt.Printf("Invalid input. Using default of %d days.\n", defaultDays)
		*days = defaultDays
		return true
	}
	*days = val
	return true
}

func buildQuery(days int) string {
	return fmt.Sprintf(
		"from:%s subject:(\"thanks for your preorder\" OR \"thanks for your order\" OR \"Canceled: delivery from order\" OR \"was canceled\" OR \"Shipped:\" OR \"Arrived:\" OR \"Delivered:\") newer_than:%dd",
		walmartSender, days,
	)
}

func formatDateRange(days int) string {
	end := time.Now()
	start := end.AddDate(0, 0, -days)
	return fmt.Sprintf("%s_to_%s", start.Format("2006-01-02"), end.Format("2006-01-02"))
}

func openReport(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("abs: %w", err)
	}
	return util.OpenBrowser(abs)
}

func discoverAccounts() []AccountConfig {
	var accounts []AccountConfig

	// Check root credentials
	if fileExists("credentials.json") {
		accounts = append(accounts, AccountConfig{
			Name:            "[Root credentials]",
			Email:           "",
			CredentialsPath: "credentials.json",
			TokenPath:       "token.json",
			IsRoot:          true,
		})
	}

	// Scan for account folders
	entries, err := os.ReadDir(".")
	if err != nil {
		return accounts
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		if strings.Contains(dirName, "@gmail.com") {
			credsPath := filepath.Join(dirName, "credentials.json")
			tokenPath := filepath.Join(dirName, "token.json")
			if fileExists(credsPath) {
				accounts = append(accounts, AccountConfig{
					Name:            dirName,
					Email:           dirName,
					CredentialsPath: credsPath,
					TokenPath:       tokenPath,
					IsRoot:          false,
				})
			}
		}
	}

	return accounts
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func hasValidToken(tokenPath string) bool {
	if !fileExists(tokenPath) {
		return false
	}
	// Try to read and parse the token file
	f, err := os.Open(tokenPath)
	if err != nil {
		return false
	}
	defer f.Close()

	var token map[string]interface{}
	if err := json.NewDecoder(f).Decode(&token); err != nil {
		return false
	}

	// Check if token has required fields
	_, hasAccess := token["access_token"]
	_, hasRefresh := token["refresh_token"]
	return hasAccess || hasRefresh
}

func promptMultiAccountMode() bool {
	fmt.Print("Scan multiple accounts and combine results? (y/n): ")
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

func mergeOrders(dest, src map[string]*report.Order) {
	for id, order := range src {
		if existing, ok := dest[id]; ok {
			// Merge logic: prefer more complete data
			if len(existing.Items) == 0 {
				existing.Items = order.Items
			}
			if existing.Total == "" {
				existing.Total = order.Total
			}
			if existing.OrderDate == "" {
				existing.OrderDate = order.OrderDate
				existing.OrderDateParsed = order.OrderDateParsed
			}
			// Keep canceled status if either is canceled
			if existing.Status != "canceled" {
				existing.Status = order.Status
			}
		} else {
			dest[id] = order
		}
	}
}

func processAccountsInParallel(accounts []AccountConfig, days int) {
	allOrders := make(map[string]*report.Order)
	var allShipped []*report.ShippedOrder
	totalEmails := 0
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, account := range accounts {
		wg.Add(1)
		go func(acc AccountConfig) {
			defer wg.Done()

			startTime := time.Now()
			fmt.Printf("\nProcessing account: %s\n", acc.Name)

			srv, err := gmail.InitializeGmailService(acc.CredentialsPath, acc.TokenPath)
			if err != nil {
				log.Printf("Error with %s: %v", acc.Name, err)
				return
			}

			// Get email if root account
			accountEmail := acc.Email
			if acc.IsRoot {
				profile, err := srv.Users.GetProfile("me").Do()
				if err != nil {
					log.Printf("Failed to get profile for %s: %v", acc.Name, err)
					return
				}
				accountEmail = profile.EmailAddress
				fmt.Printf("  → Detected email: %s\n", accountEmail)
			}

			query := buildQuery(days)
			messages, err := gmail.FetchMessages(srv, "me", query)
			if err != nil {
				log.Printf("Failed to fetch messages for %s: %v", accountEmail, err)
				return
			}

			orders, shipped, err := gmail.ProcessEmails(srv, "me", messages)
			if err != nil {
				log.Printf("Failed to process emails for %s: %v", accountEmail, err)
				return
			}

			elapsed := time.Since(startTime)
			fmt.Printf("  ✓ Completed %s in %s\n", accountEmail, elapsed.Round(time.Millisecond))

			// Thread-safe merge
			mu.Lock()
			mergeOrders(allOrders, orders)
			allShipped = append(allShipped, shipped...)
			totalEmails += len(messages)
			mu.Unlock()
		}(account)
	}

	wg.Wait()

	// Generate combined report
	outDir := "out/combined"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("failed to create output directory: %v", err)
	}

	dateRange := formatDateRange(days)
	htmlPath := filepath.Join(outDir, fmt.Sprintf("orders_%s.html", dateRange))
	csvPath := filepath.Join(outDir, fmt.Sprintf("orders_%s.csv", dateRange))
	shippedCSVPath := filepath.Join(outDir, fmt.Sprintf("shipped_orders_%s.csv", dateRange))

	if err := report.GenerateHTML(allOrders, totalEmails, days, htmlPath, allShipped); err != nil {
		log.Fatalf("write html: %v", err)
	}
	if err := report.GenerateCSV(allOrders, csvPath); err != nil {
		log.Fatalf("write csv: %v", err)
	}
	if err := report.GenerateShippedCSV(allShipped, shippedCSVPath); err != nil {
		log.Fatalf("write shipped csv: %v", err)
	}

	fmt.Printf("\nCombined report has been generated: %s\n", htmlPath)
	if err := openReport(htmlPath); err != nil {
		log.Printf("open report: %v", err)
	}
}

func processAccountsSequentially(accounts []AccountConfig, days int) {
	allOrders := make(map[string]*report.Order)
	var allShipped []*report.ShippedOrder
	totalEmails := 0

	for _, account := range accounts {
		startTime := time.Now()
		fmt.Printf("\nProcessing account: %s\n", account.Name)

		srv, err := gmail.InitializeGmailService(account.CredentialsPath, account.TokenPath)
		if err != nil {
			log.Printf("Error with %s: %v", account.Name, err)
			continue
		}

		// Get email if root account
		accountEmail := account.Email
		if account.IsRoot {
			profile, err := srv.Users.GetProfile("me").Do()
			if err != nil {
				log.Printf("Failed to get profile for %s: %v", account.Name, err)
				continue
			}
			accountEmail = profile.EmailAddress
			fmt.Printf("  → Detected email: %s\n", accountEmail)
		}

		query := buildQuery(days)
		messages, err := gmail.FetchMessages(srv, "me", query)
		if err != nil {
			log.Printf("Failed to fetch messages for %s: %v", accountEmail, err)
			continue
		}

		orders, shipped, err := gmail.ProcessEmails(srv, "me", messages)
		if err != nil {
			log.Printf("Failed to process emails for %s: %v", accountEmail, err)
			continue
		}

		elapsed := time.Since(startTime)
		fmt.Printf("  ✓ Completed %s in %s\n", accountEmail, elapsed.Round(time.Millisecond))

		mergeOrders(allOrders, orders)
		allShipped = append(allShipped, shipped...)
		totalEmails += len(messages)
	}

	// Generate combined report
	outDir := "out/combined"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("failed to create output directory: %v", err)
	}

	dateRange := formatDateRange(days)
	htmlPath := filepath.Join(outDir, fmt.Sprintf("orders_%s.html", dateRange))
	csvPath := filepath.Join(outDir, fmt.Sprintf("orders_%s.csv", dateRange))
	shippedCSVPath := filepath.Join(outDir, fmt.Sprintf("shipped_orders_%s.csv", dateRange))

	if err := report.GenerateHTML(allOrders, totalEmails, days, htmlPath, allShipped); err != nil {
		log.Fatalf("write html: %v", err)
	}
	if err := report.GenerateCSV(allOrders, csvPath); err != nil {
		log.Fatalf("write csv: %v", err)
	}
	if err := report.GenerateShippedCSV(allShipped, shippedCSVPath); err != nil {
		log.Fatalf("write shipped csv: %v", err)
	}

	fmt.Printf("\nCombined report has been generated: %s\n", htmlPath)
	if err := openReport(htmlPath); err != nil {
		log.Printf("open report: %v", err)
	}
}

func processSingleAccount(account AccountConfig, days int) {
	startTime := time.Now()

	srv, err := gmail.InitializeGmailService(account.CredentialsPath, account.TokenPath)
	if err != nil {
		log.Fatalf("unable to initialize gmail service: %v", err)
	}

	const user = "me"
	profile, err := srv.Users.GetProfile(user).Do()
	if err != nil {
		log.Fatalf("unable to retrieve user profile: %v", err)
	}

	fmt.Printf("\nProcessing account: %s\n", profile.EmailAddress)

	query := buildQuery(days)
	allMessages, err := gmail.FetchMessages(srv, user, query)
	if err != nil {
		log.Fatalf("unable to fetch messages: %v", err)
	}

	orders, shipped, err := gmail.ProcessEmails(srv, user, allMessages)
	if err != nil {
		log.Fatalf("processing failed: %v", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("  ✓ Completed %s in %s\n", profile.EmailAddress, elapsed.Round(time.Millisecond))

	outDir := filepath.Join("out", profile.EmailAddress)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("failed to create output directory: %v", err)
	}

	dateRange := formatDateRange(days)
	htmlPath := filepath.Join(outDir, fmt.Sprintf("orders_%s.html", dateRange))
	csvPath := filepath.Join(outDir, fmt.Sprintf("orders_%s.csv", dateRange))
	shippedCSVPath := filepath.Join(outDir, fmt.Sprintf("shipped_orders_%s.csv", dateRange))

	if err := report.GenerateHTML(orders, len(allMessages), days, htmlPath, shipped); err != nil {
		log.Fatalf("write html: %v", err)
	}
	if err := report.GenerateCSV(orders, csvPath); err != nil {
		log.Fatalf("write csv: %v", err)
	}
	if err := report.GenerateShippedCSV(shipped, shippedCSVPath); err != nil {
		log.Fatalf("write shipped csv: %v", err)
	}

	fmt.Printf("Report has been generated: %s\n", htmlPath)
	if err := openReport(htmlPath); err != nil {
		log.Printf("open report: %v", err)
	}
}
