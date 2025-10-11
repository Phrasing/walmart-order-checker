package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"walmart-order-checker/pkg/gmail"
	"walmart-order-checker/pkg/report"
)

func main() {
	daysToScan := flag.Int("days", 10, "Number of days back to scan for emails")
	flag.Parse()

	srv := gmail.InitializeGmailService()
	user := "me"

	profile, err := srv.Users.GetProfile(user).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve user profile: %v", err)
	}

	query := buildQuery(*daysToScan)
	allMessages := gmail.FetchMessages(srv, user, query)
	orders, shippedOrders := gmail.ProcessEmails(srv, user, allMessages)

	outDir := filepath.Join("out", profile.EmailAddress)
	if err := os.MkdirAll(outDir, 0755); err != nil && !os.IsExist(err) {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	dateRange := formatDateRange(*daysToScan)
	paths := struct {
		html       string
		csv        string
		shippedCSV string
	}{
		html:       filepath.Join(outDir, fmt.Sprintf("orders_%s.html", dateRange)),
		csv:        filepath.Join(outDir, fmt.Sprintf("orders_%s.csv", dateRange)),
		shippedCSV: filepath.Join(outDir, fmt.Sprintf("shipped_orders_%s.csv", dateRange)),
	}

	shippedSlice := mapToSlice(shippedOrders)

	report.GenerateHTML(orders, len(allMessages), *daysToScan, paths.html, shippedSlice)
	report.GenerateCSV(orders, paths.csv)
	report.GenerateShippedCSV(shippedSlice, paths.shippedCSV)

	fmt.Printf("HTML report has been generated: %s\n", paths.html)
	openReport(paths.html)
}

func buildQuery(days int) string {
	return fmt.Sprintf("from:help@walmart.com subject:(\"thanks for your preorder\" OR \"thanks for your order\" OR \"Canceled: delivery from order\" OR \"Shipped:\") newer_than:%dd", days)
}

func formatDateRange(days int) string {
	end := time.Now()
	start := end.AddDate(0, 0, -days)
	return fmt.Sprintf("%s_to_%s", start.Format("2006-01-02"), end.Format("2006-01-02"))
}

func mapToSlice(m map[string]*report.ShippedOrder) []*report.ShippedOrder {
	result := make([]*report.ShippedOrder, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	return result
}

func openReport(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		log.Printf("Could not get absolute path for %s: %v", path, err)
		return
	}
	gmail.OpenBrowser(abs)
}
