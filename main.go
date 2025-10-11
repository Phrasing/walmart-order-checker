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
	var daysToScan int
	flag.IntVar(&daysToScan, "days", 10, "Number of days back to scan for emails.")
	flag.Parse()

	srv := gmail.InitializeGmailService()

	user := "me"
	profile, err := srv.Users.GetProfile(user).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve user profile: %v", err)
	}

	query := fmt.Sprintf("from:help@walmart.com subject:(\"thanks for your preorder\" OR \"thanks for your order\" OR \"Canceled: delivery from order\" OR \"Shipped:\") newer_than:%dd", daysToScan)
	allMessages := gmail.FetchMessages(srv, user, query)
	orders, shippedOrders := gmail.ProcessEmails(srv, user, allMessages)

	outDir := fmt.Sprintf("out/%s", profile.EmailAddress)
	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		os.MkdirAll(outDir, 0755)
	}

	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -daysToScan)
	dateRangeStr := fmt.Sprintf("%s_to_%s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	htmlPath := fmt.Sprintf("%s/orders_%s.html", outDir, dateRangeStr)
	csvPath := fmt.Sprintf("%s/orders_%s.csv", outDir, dateRangeStr)
	shippedCsvPath := fmt.Sprintf("%s/shipped_orders_%s.csv", outDir, dateRangeStr)

	var shippedOrdersSlice []*report.ShippedOrder
	for _, so := range shippedOrders {
		shippedOrdersSlice = append(shippedOrdersSlice, so)
	}

	report.GenerateHTML(orders, len(allMessages), daysToScan, htmlPath, shippedOrdersSlice)
	report.GenerateCSV(orders, csvPath)
	report.GenerateShippedCSV(shippedOrdersSlice, shippedCsvPath)

	fmt.Printf("HTML report has been generated: %s\n", htmlPath)
	absPath, err := filepath.Abs(htmlPath)
	if err != nil {
		log.Printf("Could not get absolute path for %s: %v", htmlPath, err)
		return
	}
	gmail.OpenBrowser(absPath)
}
