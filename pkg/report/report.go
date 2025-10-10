package report

import (
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

func normalizeProductName(name string) string {
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

func formatOrderID(id string) string {
	if len(id) > 7 && !strings.Contains(id, "-") {
		return id[:7] + "-" + id[7:]
	}
	return id
}

func learnPrices(nonCanceledOrders []*Order) map[string]float64 {
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

func calculateSummaries(nonCanceledOrders []*Order, learnedPrices map[string]float64) map[string]*ProductSummary {
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

func calculateEmailStats(orders map[string]*Order, totalEmailsScanned int) EmailStats {
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

func calculateProductStats(orders map[string]*Order) []ProductStats {
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

func prepareOrderDetails(nonCanceledOrders []*Order, learnedPrices map[string]float64) []OrderDetail {
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
				OrderID:   formatOrderID(order.ID),
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
	dateRangeStr := fmt.Sprintf("Email Scan Range: %s to %s (%d days)", startDate.Format("Jan 2, 2006"), endDate.Format("Jan 2, 2006"), daysToScan)

	canonicalNames := make(map[string]string)
	for _, order := range orders {
		for i := range order.Items {
			normalized := normalizeProductName(order.Items[i].Name)
			if _, ok := canonicalNames[normalized]; !ok {
				canonicalNames[normalized] = order.Items[i].Name
			}
		}
	}
	for _, order := range orders {
		for i := range order.Items {
			normalized := normalizeProductName(order.Items[i].Name)
			order.Items[i].Name = canonicalNames[normalized]
		}
	}

	emailStats := calculateEmailStats(orders, totalEmailsScanned)
	stats := calculateProductStats(orders)

	var nonCanceledOrders []*Order
	for _, order := range orders {
		if order.Status != "canceled" {
			nonCanceledOrders = append(nonCanceledOrders, order)
		}
	}

	sort.Slice(nonCanceledOrders, func(i, j int) bool {
		return nonCanceledOrders[i].OrderDateParsed.Before(nonCanceledOrders[j].OrderDateParsed)
	})

	learnedPrices := learnPrices(nonCanceledOrders)
	productSummaryMap := calculateSummaries(nonCanceledOrders, learnedPrices)

	var productSummaries []ProductSummary
	for _, summary := range productSummaryMap {
		productSummaries = append(productSummaries, *summary)
	}
	sort.Slice(productSummaries, func(i, j int) bool {
		return productSummaries[i].TotalSpent > productSummaries[j].TotalSpent
	})

	orderDetails := prepareOrderDetails(nonCanceledOrders, learnedPrices)

	reportData := TemplateData{
		Stats:            stats,
		EmailStats:       emailStats,
		Orders:           orderDetails,
		ShippedOrders:    shippedOrders,
		ProductSummaries: productSummaries,
		DateRange:        dateRangeStr,
	}

	const tpl = `
<!DOCTYPE html>
<html>
<head>
<title>Walmart Orders</title>
<style>
    body { 
        font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen-Sans, Ubuntu, Cantarell, "Helvetica Neue", sans-serif;
        background-color: #f4f4f4;
        color: #333;
        margin: 0;
        padding: 20px;
        transition: background-color 0.3s, color 0.3s;
    }
    .dark-mode {
        background-color: #121212;
        color: #e0e0e0;
    }
    .container {
        max-width: 1200px;
        margin: 0 auto;
        background-color: #fff;
        padding: 20px;
        border-radius: 8px;
        box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        transition: background-color 0.3s;
        position: relative;
    }
    .dark-mode .container {
        background-color: #1e1e1e;
        box-shadow: 0 2px 4px rgba(0,0,0,0.5);
    }
    table { 
        border-collapse: collapse; 
        width: 100%; 
        margin-top: 20px;
    }
    th, td { 
        border: 1px solid #ddd; 
        padding: 8px; 
        text-align: left;
    }
    .dark-mode th, .dark-mode td {
        border-color: #444;
    }
    th { 
        background-color: #f2f2f2; 
        cursor: pointer; 
    }
    .dark-mode th {
        background-color: #333;
    }
    img { 
        max-width: 50px; 
        max-height: 50px; 
    }
    h1 {
        text-align: center;
    }
    #theme-toggle {
        position: absolute;
        top: 20px;
        right: 20px;
        background: none;
        border: none;
        font-size: 24px;
        cursor: pointer;
    }
    .tab {
        overflow: hidden;
        border-bottom: 1px solid #ccc;
    }
    .dark-mode .tab {
        border-bottom: 1px solid #444;
    }
    .tab button {
        background-color: inherit;
        float: left;
        border: none;
        outline: none;
        cursor: pointer;
        padding: 14px 16px;
        transition: 0.3s;
        font-size: 17px;
        color: inherit;
        border-bottom: 3px solid transparent;
    }
    .tab button:hover {
        opacity: 0.7;
    }
    .tab button.active {
        border-bottom: 3px solid #007bff;
    }
    .tabcontent {
        display: none;
        padding: 20px 0;
    }
</style>
</head>
<body>
<div class="container">
    <button id="theme-toggle">💡</button>
    
    <div class="tab">
        <button class="tablinks active" onclick="openTab(event, 'OverallStats')">Overall Statistics</button>
        <button class="tablinks" onclick="openTab(event, 'CancellationStats')">Cancellation Statistics</button>
        <button class="tablinks" onclick="openTab(event, 'Orders')">Non-Canceled Orders</button>
        <button class="tablinks" onclick="openTab(event, 'Shipped')">Shipped Orders</button>
    </div>

    <div id="OverallStats" class="tabcontent" style="display: block;">
        <h1>Overall Statistics</h1>
		<p style="text-align: center; font-style: italic;">{{.DateRange}}</p>
        <table>
            <thead>
                <tr>
                    <th>Total Emails Scanned</th>
                    <th>Total Unique Orders</th>
                    <th>Total Canceled Orders</th>
                    <th>Cancellation Rate</th>
                </tr>
            </thead>
            <tbody>
                <tr>
                    <td>{{.EmailStats.TotalEmailsScanned}}</td>
                    <td>{{.EmailStats.TotalOrders}}</td>
                    <td>{{.EmailStats.TotalCanceled}}</td>
                    <td>{{printf "%.2f" .EmailStats.CancellationRate}}%</td>
                </tr>
            </tbody>
        </table>
		<h2 style="margin-top: 40px;">Total Units & Spending by Product</h2>
        <table id="productSummaryTable">
            <thead>
                <tr>
                    <th>Thumbnail</th>
                    <th>Product Name</th>
                    <th onclick="sortTable(2, 'productSummaryTable')">Total Units (Non-Canceled) &#x2195;</th>
                    <th onclick="sortTable(3, 'productSummaryTable')">Price Per Unit (Estimated) &#x2195;</th>
                    <th onclick="sortTable(4, 'productSummaryTable')">Total Spent (Estimated) &#x2195;</th>
                </tr>
            </thead>
            <tbody>
                {{range .ProductSummaries}}
                <tr>
                    <td><img src="{{.ImageURL}}" alt="{{.Name}}"></td>
                    <td>{{.Name}}</td>
                    <td>{{.TotalUnits}}</td>
                    <td>${{printf "%.2f" .PricePerUnit}}</td>
                    <td>${{printf "%.2f" .TotalSpent}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>
        <p style="font-size: 12px; text-align: center; margin-top: 10px;">
            * Total Spent and Price Per Unit are estimated based on the total cost of orders containing only that specific product. Orders with multiple different products are excluded from these calculations.
        </p>
    </div>

    <div id="CancellationStats" class="tabcontent">
        <h1>Cancellation Statistics by Product</h1>
        <table>
            <thead>
                <tr>
                    <th>Thumbnail</th>
                    <th>Product Name</th>
                    <th>Total Ordered</th>
                    <th>Total Canceled</th>
                    <th>Cancellation Rate</th>
                </tr>
            </thead>
            <tbody>
                {{range .Stats}}
                <tr>
                    <td><img src="{{.ImageURL}}" alt="{{.Name}}"></td>
                    <td>{{.Name}}</td>
                    <td>{{.TotalOrdered}}</td>
                    <td>{{.TotalCanceled}}</td>
                    <td>{{printf "%.2f" .CancelRate}}%</td>
                </tr>
                {{end}}
            </tbody>
        </table>
    </div>

    <div id="Orders" class="tabcontent">
        <h1>Non-Canceled Walmart Orders</h1>
        <table id="ordersTable">
    <thead>
        <tr>
            <th onclick="sortTable(0, 'ordersTable')">Order Date &#x2195;</th>
            <th>Order #</th>
            <th>Thumbnail</th>
            <th>Product Name</th>
            <th>Quantity</th>
            <th>Line Total (Est.)</th>
        </tr>
    </thead>
    <tbody>
        {{range .Orders}}
        <tr>
            <td>{{.OrderDate}}</td>
            <td>{{.OrderID}}</td>
            <td><img src="{{.ImageURL}}" alt="{{.Name}}"></td>
            <td>{{.Name}}</td>
            <td>{{.Quantity}}</td>
            <td>{{.Total}}</td>
        </tr>
        {{end}}
    </tbody>
</table>
    </div>

    <div id="Shipped" class="tabcontent">
        <h1>Shipped Orders</h1>
        <button onclick="exportTableToCSV('shippedTable', 'shipped-orders.csv')">Export to CSV</button>
        <table id="shippedTable">
    <thead>
        <tr>
            <th>Order #</th>
            <th>Carrier</th>
            <th>Tracking #</th>
            <th>Estimated Arrival</th>
        </tr>
    </thead>
    <tbody>
        {{range .ShippedOrders}}
        <tr>
            <td>{{.ID}}</td>
            <td>{{.Carrier}}</td>
            <td>{{.TrackingNumber}}</td>
            <td>{{.EstimatedArrival}}</td>
        </tr>
        {{end}}
    </tbody>
</table>
    </div>
</div>
<script>
function exportTableToCSV(tableId, filename) {
    var csv = [];
    var rows = document.querySelectorAll("#" + tableId + " tr");
    
    for (var i = 0; i < rows.length; i++) {
        var row = [], cols = rows[i].querySelectorAll("td, th");
        
        for (var j = 0; j < cols.length; j++) 
            row.push(cols[j].innerText);
        
        csv.push(row.join(","));        
    }

    // Download CSV file
    var csvFile = new Blob([csv.join("\n")], {type: "text/csv"});
    var downloadLink = document.createElement("a");
    downloadLink.download = filename;
    downloadLink.href = window.URL.createObjectURL(csvFile);
    downloadLink.style.display = "none";
    document.body.appendChild(downloadLink);
    downloadLink.click();
}

function openTab(evt, tabName) {
    var i, tabcontent, tablinks;
    tabcontent = document.getElementsByClassName("tabcontent");
    for (i = 0; i < tabcontent.length; i++) {
        tabcontent[i].style.display = "none";
    }
    tablinks = document.getElementsByClassName("tablinks");
    for (i = 0; i < tablinks.length; i++) {
        tablinks[i].className = tablinks[i].className.replace(" active", "");
    }
    document.getElementById(tabName).style.display = "block";
    evt.currentTarget.className += " active";
}

function sortTable(n, tableId) {
  var table, rows, switching, i, x, y, shouldSwitch, dir, switchcount = 0;
  table = document.getElementById(tableId);
  switching = true;
  dir = "asc"; 
  while (switching) {
    switching = false;
    rows = table.rows;
    for (i = 1; i < (rows.length - 1); i++) {
      shouldSwitch = false;
      x = rows[i].getElementsByTagName("TD")[n];
      y = rows[i + 1].getElementsByTagName("TD")[n];
      
      var xContent = x.innerHTML;
      var yContent = y.innerHTML;

      var xVal, yVal;

      if (tableId === 'ordersTable' && n === 0) {
          xVal = new Date(xContent);
          yVal = new Date(yContent);
      } else {
          xVal = parseFloat(xContent.replace(/[^0-9.-]+/g,""));
          yVal = parseFloat(yContent.replace(/[^0-9.-]+/g,""));
      }

      if (dir == "asc") {
        if (xVal > yVal) {
          shouldSwitch = true;
          break;
        }
      } else if (dir == "desc") {
        if (xVal < yVal) {
          shouldSwitch = true;
          break;
        }
      }
    }
    if (shouldSwitch) {
      rows[i].parentNode.insertBefore(rows[i + 1], rows[i]);
      switching = true;
      switchcount ++;      
    } else {
      if (switchcount == 0 && dir == "asc") {
        dir = "desc";
        switching = true;
      }
    }
  }
}

const themeToggle = document.getElementById('theme-toggle');

function setTheme(theme) {
    if (theme === 'dark-mode') {
        document.body.classList.add('dark-mode');
        themeToggle.innerHTML = '☀️';
    } else {
        document.body.classList.remove('dark-mode');
        themeToggle.innerHTML = '💡';
    }
    localStorage.setItem('theme', theme);
}

let currentTheme = localStorage.getItem('theme');
if (!currentTheme) {
    currentTheme = 'dark-mode';
}
setTheme(currentTheme);

themeToggle.addEventListener('click', () => {
    let theme = document.body.classList.contains('dark-mode') ? 'light-mode' : 'dark-mode';
    setTheme(theme);
});
</script>
</body>
</html>`

	t, err := template.New("webpage").Parse(tpl)
	if err != nil {
		log.Fatalf("Failed to parse HTML template: %v", err)
	}

	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("Failed to create HTML file: %v", err)
	}
	defer f.Close()

	err = t.Execute(f, reportData)
	if err != nil {
		log.Fatalf("Failed to execute template: %v", err)
	}
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
				writer.Write([]string{formatOrderID(order.ID), order.OrderDate, order.Total, item.Name, fmt.Sprintf("%d", item.Quantity)})
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
		writer.Write([]string{formatOrderID(order.ID), order.Carrier, order.TrackingNumber, order.EstimatedArrival})
	}
	fmt.Printf("\nShipped orders have been written to %s\n", path)
}
