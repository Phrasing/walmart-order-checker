package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func main() {
	daysFlag := flag.Int("days", defaultDays, "Number of days back to scan for emails")
	flag.Parse()

	maybePromptDays(daysFlag)
	days := *daysFlag

	srv, err := gmail.InitializeGmailService()
	if err != nil {
		log.Fatalf("unable to initialize gmail service: %v", err)
	}

	const user = "me"
	profile, err := srv.Users.GetProfile(user).Do()
	if err != nil {
		log.Fatalf("unable to retrieve user profile: %v", err)
	}

	query := buildQuery(days)
	allMessages, err := gmail.FetchMessages(srv, user, query)
	if err != nil {
		log.Fatalf("unable to fetch messages: %v", err)
	}

	orders, shipped, err := gmail.ProcessEmails(srv, user, allMessages)
	if err != nil {
		log.Fatalf("processing failed: %v", err)
	}

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
		"from:%s subject:(\"thanks for your preorder\" OR \"thanks for your order\" OR \"Canceled: delivery from order\" OR \"Shipped:\") newer_than:%dd",
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
