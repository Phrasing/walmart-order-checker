package report

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"html/template"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed template.html
var templateHTML string

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

type ShippedOrder struct {
	ID               string
	TrackingNumber   string
	Carrier          string
	EstimatedArrival string
}

type Item struct {
	Name     string
	Quantity int
	ImageURL string
}

type ProductStats struct {
	Name          string
	ImageURL      string
	TotalOrdered  int
	TotalCanceled int
	CancelRate    float64
}

type EmailStats struct {
	TotalEmailsScanned int
	TotalOrders        int
	TotalCanceled      int
	CancellationRate   float64
}

type TemplateData struct {
	Stats            []ProductStats
	EmailStats       EmailStats
	Orders           []OrderDetail
	ShippedOrders    []*ShippedOrder
	ProductSummaries []ProductSummary
	DateRange        string
}

type OrderDetail struct {
	OrderID   string
	OrderDate string
	ImageURL  string
	Name      string
	Quantity  int
	Total     string
}

type ProductSummary struct {
	Name         string
	ImageURL     string
	TotalUnits   int
	TotalSpent   float64
	PricePerUnit float64
}

var nonAlnum = regexp.MustCompile("[^a-z0-9]+")

func NormalizeProductName(name string) string {
	normalized := strings.ToLower(name)
	normalized = strings.ReplaceAll(normalized, "pokemon trading card games", "")
	normalized = strings.ReplaceAll(normalized, "pokemon", "")
	normalized = strings.ReplaceAll(normalized, "scarlett violet", "sv")
	normalized = strings.ReplaceAll(normalized, "evolutions", "evo")
	normalized = strings.ReplaceAll(normalized, "suprise", "surprise")
	normalized = nonAlnum.ReplaceAllString(normalized, "")
	return normalized
}

func FormatOrderID(id string) string {
	if len(id) > 7 && !strings.Contains(id, "-") {
		return id[:7] + "-" + id[7:]
	}
	return id
}

func LearnPrices(nonCanceledOrders []*Order) map[string]float64 {
	learned := make(map[string]float64)
	for _, order := range nonCanceledOrders {
		if len(order.Items) == 0 {
			continue
		}
		same := true
		first := order.Items[0].Name
		for i := 1; i < len(order.Items); i++ {
			if order.Items[i].Name != first {
				same = false
				break
			}
		}
		if !same {
			continue
		}
		if _, ok := learned[first]; ok {
			continue
		}
		totalString := strings.ReplaceAll(order.Total, "$", "")
		totalString = strings.ReplaceAll(totalString, ",", "")
		totalFloat, err := strconv.ParseFloat(totalString, 64)
		if err != nil {
			continue
		}
		totalQty := 0
		for _, item := range order.Items {
			totalQty += item.Quantity
		}
		if totalQty > 0 {
			learned[first] = totalFloat / float64(totalQty)
		}
	}
	return learned
}

func CalculateSummaries(nonCanceledOrders []*Order, learnedPrices map[string]float64) map[string]*ProductSummary {
	m := make(map[string]*ProductSummary)
	for _, order := range nonCanceledOrders {
		for _, item := range order.Items {
			if _, ok := m[item.Name]; !ok {
				m[item.Name] = &ProductSummary{Name: item.Name, ImageURL: item.ImageURL}
			}
			s := m[item.Name]
			s.TotalUnits += item.Quantity
			if price, ok := learnedPrices[item.Name]; ok {
				s.PricePerUnit = price
				s.TotalSpent = price * float64(s.TotalUnits)
			}
		}
	}
	return m
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
		cancelRate = float64(totalCanceled) / float64(totalOrders) * 100
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
			stat.CancelRate = float64(stat.TotalCanceled) / float64(stat.TotalOrdered) * 100
		}
		stats = append(stats, *stat)
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].CancelRate > stats[j].CancelRate })
	return stats
}

func PrepareOrderDetails(nonCanceledOrders []*Order, learnedPrices map[string]float64) []OrderDetail {
	var out []OrderDetail
	for _, order := range nonCanceledOrders {
		for _, item := range order.Items {
			totalStr := order.Total
			if price, ok := learnedPrices[item.Name]; ok {
				totalStr = fmt.Sprintf("$%.2f", price*float64(item.Quantity))
			}
			out = append(out, OrderDetail{
				OrderID:   FormatOrderID(order.ID),
				OrderDate: order.OrderDate,
				ImageURL:  item.ImageURL,
				Name:      item.Name,
				Quantity:  item.Quantity,
				Total:     totalStr,
			})
		}
	}
	return out
}

func GenerateHTML(orders map[string]*Order, totalEmailsScanned int, daysToScan int, path string, shippedOrders []*ShippedOrder) error {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -daysToScan)
	dateRangeStr := fmt.Sprintf(
		"Email Scan Range: %s to %s (%d days)",
		startDate.Format("Jan 2, 2006"),
		endDate.Format("Jan 2, 2006"),
		daysToScan,
	)

	normalizeProductNames(orders)
	emailStats := CalculateEmailStats(orders, totalEmailsScanned)
	stats := CalculateProductStats(orders)
	nonCanceled := filterNonCanceled(orders)
	learned := LearnPrices(nonCanceled)
	productSummaries := buildProductSummaries(nonCanceled, learned)
	orderDetails := PrepareOrderDetails(nonCanceled, learned)

	data := TemplateData{
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
		return fmt.Errorf("create html: %w", err)
	}
	defer f.Close()
	if err := t.Execute(f, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}
	return nil
}

func normalizeProductNames(orders map[string]*Order) {
	canonical := make(map[string]string)
	for _, order := range orders {
		for i := range order.Items {
			norm := NormalizeProductName(order.Items[i].Name)
			if _, ok := canonical[norm]; !ok {
				canonical[norm] = order.Items[i].Name
			}
		}
	}
	for _, order := range orders {
		for i := range order.Items {
			norm := NormalizeProductName(order.Items[i].Name)
			order.Items[i].Name = canonical[norm]
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
	m := CalculateSummaries(orders, learnedPrices)
	out := make([]ProductSummary, 0, len(m))
	for _, s := range m {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TotalSpent > out[j].TotalSpent })
	return out
}

func GenerateCSV(orders map[string]*Order, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"Order ID", "Order Date", "Order Total", "Item Name", "Quantity"}); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	for _, order := range orders {
		if order.Status == "canceled" {
			continue
		}
		for _, item := range order.Items {
			rec := []string{
				FormatOrderID(order.ID),
				order.OrderDate,
				order.Total,
				item.Name,
				fmt.Sprintf("%d", item.Quantity),
			}
			if err := w.Write(rec); err != nil {
				return fmt.Errorf("write row: %w", err)
			}
		}
	}

	if err := w.Error(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}

	return nil
}

func GenerateShippedCSV(shippedOrders []*ShippedOrder, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"Order ID", "Carrier", "Tracking #", "Estimated Arrival"}); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	for _, order := range shippedOrders {
		rec := []string{
			FormatOrderID(order.ID),
			order.Carrier,
			order.TrackingNumber,
			order.EstimatedArrival,
		}
		if err := w.Write(rec); err != nil {
			return fmt.Errorf("write row: %w", err)
		}
	}
	if err := w.Error(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	return nil
}
