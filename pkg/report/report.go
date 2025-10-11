package report

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"html/template"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed template.html
var templateHTML string

// Order represents a Walmart order, including its ID, items, total cost, and status.
type Order struct {
	ID               string
	Items            []Item
	Total            string
	OrderDate        string
	OrderDateParsed  time.Time
	Status           string
	TrackingNumber   string
	Carrier          string
	EstimatedArrival string
}

// ShippedOrder represents a shipped order with its tracking information.
type ShippedOrder struct {
	ID               string
	TrackingNumber   string
	Carrier          string
	EstimatedArrival string
}

// Item represents a single item within a Walmart order.
type Item struct {
	Name     string
	Quantity int
	ImageURL string
}

// ProductStats holds cancellation statistics for a product.
type ProductStats struct {
	Name          string
	ImageURL      string
	TotalOrdered  int
	TotalCanceled int
	CancelRate    float64
}

// EmailStats holds overall statistics about the email scan.
type EmailStats struct {
	TotalEmailsScanned int
	TotalOrders        int
	TotalCanceled      int
	CancellationRate   float64
}

// TemplateData is the struct passed to the HTML template.
type TemplateData struct {
	Stats            []ProductStats
	EmailStats       EmailStats
	Orders           []OrderDetail
	ShippedOrders    []*ShippedOrder
	ProductSummaries []ProductSummary
	DateRange        string
}

// OrderDetail represents a single order item for display in the HTML report.
type OrderDetail struct {
	OrderID   string
	OrderDate string
	ImageURL  string
	Name      string
	Quantity  int
	Total     string
}

// ProductSummary provides a summary of spending and units for a single product.
type ProductSummary struct {
	Name         string
	ImageURL     string
	TotalUnits   int
	TotalSpent   float64
	PricePerUnit float64
}

func NormalizeProductName(name string) string {
	normalized := strings.ToLower(name)
	normalized = strings.ReplaceAll(normalized, "pokemon trading card games", "")
	normalized = strings.ReplaceAll(normalized, "pokemon", "")
	normalized = strings.ReplaceAll(normalized, "scarlett violet", "sv")
	normalized = strings.ReplaceAll(normalized, "evolutions", "evo")
	normalized = strings.ReplaceAll(normalized, "suprise", "surprise")
	reg, _ := regexp.Compile("[^a-z0-9]+")
	normalized = reg.ReplaceAllString(normalized, "")
	return normalized
}

func FormatOrderID(id string) string {
	if len(id) > 7 && !strings.Contains(id, "-") {
		return id[:7] + "-" + id[7:]
	}
	return id
}

func LearnPrices(nonCanceledOrders []*Order) map[string]float64 {
	learnedPrices := make(map[string]float64)
	for _, order := range nonCanceledOrders {
		if len(order.Items) > 0 {
			isSingleItemType := true
			firstItemName := order.Items[0].Name
			for i := 1; i < len(order.Items); i++ {
				if order.Items[i].Name != firstItemName {
					isSingleItemType = false
					break
				}
			}

			if isSingleItemType {
				if _, ok := learnedPrices[firstItemName]; !ok {
					totalString := strings.ReplaceAll(order.Total, "$", "")
					totalString = strings.ReplaceAll(totalString, ",", "")
					if totalFloat, err := strconv.ParseFloat(totalString, 64); err == nil {
						totalQuantity := 0
						for _, item := range order.Items {
							totalQuantity += item.Quantity
						}
						if totalQuantity > 0 {
							learnedPrices[firstItemName] = totalFloat / float64(totalQuantity)
						}
					}
				}
			}
		}
	}
	return learnedPrices
}

func CalculateSummaries(nonCanceledOrders []*Order, learnedPrices map[string]float64) map[string]*ProductSummary {
	productSummaryMap := make(map[string]*ProductSummary)
	for _, order := range nonCanceledOrders {
		for _, item := range order.Items {
			if _, ok := productSummaryMap[item.Name]; !ok {
				productSummaryMap[item.Name] = &ProductSummary{
					Name:     item.Name,
					ImageURL: item.ImageURL,
				}
			}
			summary := productSummaryMap[item.Name]
			summary.TotalUnits += item.Quantity
			if price, ok := learnedPrices[item.Name]; ok {
				summary.PricePerUnit = price
				summary.TotalSpent = price * float64(summary.TotalUnits)
			}
		}
	}
	return productSummaryMap
}

func CalculateEmailStats(orders map[string]*Order, totalEmailsScanned int) EmailStats {
	totalOrders := len(orders)
	totalCanceled := 0
	for _, order := range orders {
		if order.Status == "canceled" {
			totalCanceled++
		}
	}
	var cancelRate float64
	if totalOrders > 0 {
		cancelRate = (float64(totalCanceled) / float64(totalOrders)) * 100
	}
	return EmailStats{
		TotalEmailsScanned: totalEmailsScanned,
		TotalOrders:        totalOrders,
		TotalCanceled:      totalCanceled,
		CancellationRate:   cancelRate,
	}
}

func CalculateProductStats(orders map[string]*Order) []ProductStats {
	statsMap := make(map[string]*ProductStats)
	for _, order := range orders {
		for _, item := range order.Items {
			if _, ok := statsMap[item.Name]; !ok {
				statsMap[item.Name] = &ProductStats{Name: item.Name, ImageURL: item.ImageURL}
			}
			statsMap[item.Name].TotalOrdered += item.Quantity
			if order.Status == "canceled" {
				statsMap[item.Name].TotalCanceled += item.Quantity
			}
		}
	}

	var stats []ProductStats
	for _, stat := range statsMap {
		if stat.TotalOrdered > 0 {
			stat.CancelRate = (float64(stat.TotalCanceled) / float64(stat.TotalOrdered)) * 100
		}
		stats = append(stats, *stat)
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].CancelRate > stats[j].CancelRate
	})
	return stats
}

func PrepareOrderDetails(nonCanceledOrders []*Order, learnedPrices map[string]float64) []OrderDetail {
	var orderDetails []OrderDetail
	for _, order := range nonCanceledOrders {
		for _, item := range order.Items {
			var totalStr string
			if price, ok := learnedPrices[item.Name]; ok {
				totalStr = fmt.Sprintf("$%.2f", price*float64(item.Quantity))
			} else {
				totalStr = order.Total // Fallback for orders with unknown prices
			}

			orderDetails = append(orderDetails, OrderDetail{
				OrderID:   FormatOrderID(order.ID),
				OrderDate: order.OrderDate,
				ImageURL:  item.ImageURL,
				Name:      item.Name,
				Quantity:  item.Quantity,
				Total:     totalStr,
			})
		}
	}
	return orderDetails
}

func GenerateHTML(orders map[string]*Order, totalEmailsScanned int, daysToScan int, path string, shippedOrders []*ShippedOrder) {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -daysToScan)
	dateRangeStr := fmt.Sprintf("Email Scan Range: %s to %s (%d days)",
		startDate.Format("Jan 2, 2006"), endDate.Format("Jan 2, 2006"), daysToScan)

	normalizeProductNames(orders)

	emailStats := CalculateEmailStats(orders, totalEmailsScanned)
	stats := CalculateProductStats(orders)
	nonCanceledOrders := filterNonCanceled(orders)
	learnedPrices := LearnPrices(nonCanceledOrders)
	productSummaries := buildProductSummaries(nonCanceledOrders, learnedPrices)
	orderDetails := PrepareOrderDetails(nonCanceledOrders, learnedPrices)

	reportData := TemplateData{
		Stats:            stats,
		EmailStats:       emailStats,
		Orders:           orderDetails,
		ShippedOrders:    shippedOrders,
		ProductSummaries: productSummaries,
		DateRange:        dateRangeStr,
	}

	t := template.Must(template.New("webpage").Parse(templateHTML))

	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("Failed to create HTML file: %v", err)
	}

	defer f.Close()

	if err := t.Execute(f, reportData); err != nil {
		log.Fatalf("Failed to execute template: %v", err)
	}
}

func normalizeProductNames(orders map[string]*Order) {
	canonicalNames := make(map[string]string)
	for _, order := range orders {
		for i := range order.Items {
			normalized := NormalizeProductName(order.Items[i].Name)
			if _, ok := canonicalNames[normalized]; !ok {
				canonicalNames[normalized] = order.Items[i].Name
			}
		}
	}
	for _, order := range orders {
		for i := range order.Items {
			normalized := NormalizeProductName(order.Items[i].Name)
			order.Items[i].Name = canonicalNames[normalized]
		}
	}
}

func filterNonCanceled(orders map[string]*Order) []*Order {
	var result []*Order
	for _, order := range orders {
		if order.Status != "canceled" {
			result = append(result, order)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].OrderDateParsed.Before(result[j].OrderDateParsed)
	})
	return result
}

func buildProductSummaries(orders []*Order, learnedPrices map[string]float64) []ProductSummary {
	summaryMap := CalculateSummaries(orders, learnedPrices)
	summaries := make([]ProductSummary, 0, len(summaryMap))
	for _, summary := range summaryMap {
		summaries = append(summaries, *summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].TotalSpent > summaries[j].TotalSpent
	})
	return summaries
}

func GenerateCSV(orders map[string]*Order, path string) {
	file, err := os.Create(path)
	if err != nil {
		log.Fatalf("Failed to create CSV file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"Order ID", "Order Date", "Order Total", "Item Name", "Quantity"})
	for _, order := range orders {
		if order.Status != "canceled" {
			for _, item := range order.Items {
				writer.Write([]string{FormatOrderID(order.ID), order.OrderDate, order.Total, item.Name, fmt.Sprintf("%d", item.Quantity)})
			}
		}
	}
	fmt.Printf("\nNon-canceled orders have been written to %s\n", path)
}

func GenerateShippedCSV(shippedOrders []*ShippedOrder, path string) {
	file, err := os.Create(path)
	if err != nil {
		log.Fatalf("Failed to create CSV file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{"Order ID", "Carrier", "Tracking #", "Estimated Arrival"})
	for _, order := range shippedOrders {
		writer.Write([]string{FormatOrderID(order.ID), order.Carrier, order.TrackingNumber, order.EstimatedArrival})
	}
	fmt.Printf("\nShipped orders have been written to %s\n", path)
}
