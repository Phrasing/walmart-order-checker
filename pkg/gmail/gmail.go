package gmail

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"walmart-order-checker/pkg/util"

	"regexp"
	"walmart-order-checker/pkg/report"

	"github.com/PuerkitoBio/goquery"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gm "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

var (
	carrierRe   = regexp.MustCompile(`(\w+)\s+tracking\s+number`)
	orderDateRe = regexp.MustCompile(`Order date:\s*(.*)`)
)

func findHTMLPart(part *gm.MessagePart) string {
	if part == nil {
		return ""
	}
	if part.MimeType == "text/html" && part.Body != nil && part.Body.Size > 0 {
		return part.Body.Data
	}
	if strings.HasPrefix(part.MimeType, "multipart/") {
		for _, sub := range part.Parts {
			if html := findHTMLPart(sub); html != "" {
				return html
			}
		}
	}
	return ""
}

func decodeBase64(data string) (string, error) {
	trim := strings.TrimSpace(data)
	trim = strings.ReplaceAll(trim, "\n", "")
	trim = strings.ReplaceAll(trim, "\r", "")

	// Gmail payloads are typically URL-safe without padding.
	if dec, err := base64.RawURLEncoding.DecodeString(trim); err == nil {
		return string(dec), nil
	}
	// Try standard enc without padding.
	if dec, err := base64.RawStdEncoding.DecodeString(trim); err == nil {
		return string(dec), nil
	}
	// Add padding to multiple of 4 and try both.
	if m := len(trim) % 4; m != 0 {
		trim += strings.Repeat("=", 4-m)
	}
	if dec, err := base64.URLEncoding.DecodeString(trim); err == nil {
		return string(dec), nil
	}
	if dec, err := base64.StdEncoding.DecodeString(trim); err == nil {
		return string(dec), nil
	}
	return "", errors.New("base64 decode failed")
}

func getClient(config *oauth2.Config) (*http.Client, error) {
	const tokFile = "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok, err = getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		if err := saveToken(tokFile, tok); err != nil {
			return nil, err
		}
	}
	return config.Client(context.Background(), tok), nil
}

func startOAuthWebServer(authURL string) (string, error) {
	codeChan := make(chan string, 1)

	const listenAddr = "127.0.0.1:80"
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return "", fmt.Errorf("listen on %s: %w", listenAddr, err)
	}
	addr := ln.Addr().String()

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}

	var once sync.Once
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code != "" {
			once.Do(func() {
				_, _ = fmt.Fprint(w, "Authorization successful! You can close this window.")
				codeChan <- code
				go func() {
					_ = srv.Shutdown(context.Background())
				}()
			})
		} else {
			http.Redirect(w, r, authURL, http.StatusFound)
		}
	})

	go func() {
		_ = srv.Serve(ln)
	}()

	if err := util.OpenBrowser("http://" + addr); err != nil {
		log.Printf("open browser failed: %v; navigate to: %s", err, "http://"+addr)
	}

	code := <-codeChan
	return code, nil
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	// Note: uses local redirect server on random port; user is redirected to OAuth URL then back.
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Println("Attempting to open the authorization link in your browser.")
	fmt.Printf("If it doesn't open automatically, please go to this link:\n%v\n", authURL)

	code, err := startOAuthWebServer(authURL)
	if err != nil {
		return nil, err
	}
	tok, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("exchange token: %w", err)
	}
	return tok, nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var tok oauth2.Token
	if err := json.NewDecoder(f).Decode(&tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func saveToken(path string, token *oauth2.Token) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

func InitializeGmailService() (*gm.Service, error) {
	credentials, err := os.ReadFile("credentials.json")
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Error: credentials.json not found.")
			fmt.Println("\nPlease follow these steps to set up your credentials:")
			fmt.Println("1. Go to the Google Cloud Console and create a new project.")
			fmt.Println("2. Enable the Gmail API for your project.")
			fmt.Println("3. Create an OAuth 2.0 Client ID for a 'Desktop app'.")
			fmt.Println("4. Download the JSON file, rename it to 'credentials.json',")
			fmt.Println("   and place it in the same directory as this executable.")
			fmt.Println("\nPress 'Enter' to exit.")
			bufio.NewReader(os.Stdin).ReadBytes('\n')
			return nil, errors.New("credentials.json not found, please set it up")
		}
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	config, err := google.ConfigFromJSON(credentials, gm.GmailReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	client, err := getClient(config)
	if err != nil {
		return nil, err
	}
	srv, err := gm.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("gmail service: %w", err)
	}
	return srv, nil
}

func FetchMessages(srv *gm.Service, user, query string) ([]*gm.Message, error) {
	var all []*gm.Message
	var pageToken string
	for {
		req := srv.Users.Messages.List(user).Q(query)
		if pageToken != "" {
			req.PageToken(pageToken)
		}
		r, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("list messages: %w", err)
		}
		all = append(all, r.Messages...)
		if r.NextPageToken == "" {
			break
		}
		pageToken = r.NextPageToken
	}
	return all, nil
}

func processCanceledEmail(subject string, orders map[string]*report.Order) {
	parts := strings.Split(subject, "#")
	if len(parts) <= 1 {
		return
	}
	orderID := parts[1]
	if existing, ok := orders[orderID]; ok {
		existing.Status = "canceled"
	} else {
		orders[orderID] = &report.Order{ID: orderID, Status: "canceled"}
	}
}

func processPaymentFailCancelEmail(msg *gm.Message, orders map[string]*report.Order) {
	doc, err := parseMessageHTML(msg)
	if err != nil {
		return
	}
	// Extract order ID from the HTML body (format: 2000131-89912005)
	orderIDRaw := strings.TrimSpace(doc.Find("a[aria-label*=' ']").First().Text())
	if orderIDRaw == "" {
		return
	}
	// Remove hyphens to normalize format
	orderID := strings.ReplaceAll(orderIDRaw, "-", "")
	if existing, ok := orders[orderID]; ok {
		existing.Status = "canceled"
	} else {
		orders[orderID] = &report.Order{ID: orderID, Status: "canceled"}
	}
}

func processDeliveredEmail(msg *gm.Message) string {
	doc, err := parseMessageHTML(msg)
	if err != nil {
		return ""
	}
	// Delivered emails have order number in format: #2000129-05242992
	// Find the order number link with # prefix (delivered emails don't use aria-label)
	orderIDRaw := ""
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if strings.HasPrefix(text, "#") && strings.Contains(text, "-") {
			// Make sure it looks like an order number (starts with #2)
			if len(text) > 10 && text[1] == '2' {
				orderIDRaw = text
			}
		}
	})

	if orderIDRaw == "" {
		return ""
	}

	// Remove # prefix and hyphens to normalize format
	orderID := strings.TrimPrefix(orderIDRaw, "#")
	orderID = strings.ReplaceAll(orderID, "-", "")
	return orderID
}

func processShippedEmail(msg *gm.Message) []*report.ShippedOrder {
	doc, err := parseMessageHTML(msg)
	if err != nil {
		return nil
	}
	return extractShippingInfo(doc)
}

func parseMessageHTML(msg *gm.Message) (*goquery.Document, error) {
	body := findHTMLPart(msg.Payload)
	if body == "" {
		return nil, fmt.Errorf("html part not found")
	}
	decoded, err := decodeBase64(body)
	if err != nil {
		return nil, err
	}
	return goquery.NewDocumentFromReader(strings.NewReader(decoded))
}

func extractShippingInfo(doc *goquery.Document) []*report.ShippedOrder {
	orderID := strings.ReplaceAll(strings.TrimSpace(doc.Find("a[aria-label*=' ']").First().Text()), "-", "")
	var shippedOrders []*report.ShippedOrder

	var trackingNumbers []string
	doc.Find("span:contains('tracking number') a").Each(func(i int, s *goquery.Selection) {
		trackingNumbers = append(trackingNumbers, strings.TrimSpace(s.Text()))
	})

	var arrivalDates []string
	doc.Find("strong:contains('Arrives')").Each(func(i int, s *goquery.Selection) {
		arrivalDates = append(arrivalDates, s.Text())
	})

	carrier := extractCarrier(doc)

	// Pair up tracking numbers and arrival dates.
	// This assumes a 1:1 correspondence and order, which is typical for these emails.
	count := min(len(arrivalDates), len(trackingNumbers))

	for i := range count {
		if trackingNumbers[i] == "" {
			continue
		}
		shippedOrders = append(shippedOrders, &report.ShippedOrder{
			ID:               orderID,
			TrackingNumber:   trackingNumbers[i],
			Carrier:          carrier,
			EstimatedArrival: arrivalDates[i],
		})
	}

	return shippedOrders
}

func extractCarrier(doc *goquery.Document) string {
	carrierText := doc.Find("span:contains('tracking number')").Text()
	if m := carrierRe.FindStringSubmatch(carrierText); len(m) > 1 {
		return m[1]
	}
	return ""
}

func extractOrderInfo(doc *goquery.Document, subject string) *report.Order {
	orderID := strings.ReplaceAll(strings.TrimSpace(doc.Find("a[aria-label*=' ']").First().Text()), "-", "")
	orderDate, parsedDate := extractOrderDate(doc)
	return &report.Order{
		ID:              orderID,
		Items:           extractItems(doc),
		Total:           extractTotal(doc),
		OrderDate:       orderDate,
		OrderDateParsed: parsedDate,
		Status:          determineStatus(subject),
	}
}

func extractOrderDate(doc *goquery.Document) (string, time.Time) {
	dateText := doc.Find("div:contains('Order date:')").Text()
	m := orderDateRe.FindStringSubmatch(dateText)
	if len(m) <= 1 {
		return "", time.Time{}
	}
	orderDate := strings.TrimSpace(m[1])
	parsed, err := time.Parse("Mon, Jan 2, 2006", orderDate)
	if err != nil {
		return orderDate, time.Time{}
	}
	return orderDate, parsed
}

func extractTotal(doc *goquery.Document) string {
	return doc.Find("strong:contains('Includes all fees, taxes, discounts and driver tip')").
		Parent().
		Next().
		Find("strong").
		Text()
}

func extractItems(doc *goquery.Document) []report.Item {
	var items []report.Item
	doc.Find("img[alt*='quantity']").Each(func(i int, s *goquery.Selection) {
		if item, ok := parseItemFromImage(s); ok {
			items = append(items, item)
		}
	})
	return items
}

func parseItemFromImage(s *goquery.Selection) (report.Item, bool) {
	alt := s.AttrOr("alt", "")
	parts := strings.Split(alt, " item ")
	if len(parts) != 2 {
		return report.Item{}, false
	}
	qty := 1
	qtyParts := strings.Split(parts[0], " ")
	if len(qtyParts) > 1 {
		_, _ = fmt.Sscanf(qtyParts[1], "%d", &qty)
	}
	imageURL := s.AttrOr("src", "")
	if imageURL != "" {
		imageURL = fmt.Sprintf("https://images.weserv.nl/?url=%s&trim=10&bg=00000000", imageURL)
	}
	return report.Item{
		Name:     parts[1],
		Quantity: qty,
		ImageURL: imageURL,
	}, true
}

func determineStatus(subject string) string {
	if strings.Contains(subject, "preorder") {
		return "pre-ordered"
	}
	return "confirmed"
}

func mergeOrCreateOrder(orders map[string]*report.Order, newOrder *report.Order) {
	if existing, ok := orders[newOrder.ID]; ok {
		if len(existing.Items) == 0 {
			existing.Items = newOrder.Items
		}
		if existing.Status != "canceled" {
			existing.Status = newOrder.Status
		}
		return
	}
	orders[newOrder.ID] = newOrder
}

func getSubject(headers []*gm.MessagePartHeader) string {
	for _, h := range headers {
		if h.Name == "Subject" {
			return h.Value
		}
	}
	return ""
}

func ProcessEmails(srv *gm.Service, user string, allMessages []*gm.Message) (map[string]*report.Order, []*report.ShippedOrder, error) {
	orders := make(map[string]*report.Order)
	var shipped []*report.ShippedOrder
	shippedIDs := make(map[string]struct{})

	var mu sync.Mutex
	bar := progressbar.NewOptions(
		len(allMessages),
		progressbar.OptionSetDescription("Processing emails"),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionClearOnFinish(), // clears the bar line
	)

	const workers = 8
	jobs := make(chan string, workers*2)
	var wg sync.WaitGroup

	getMessage := func(id string) (*gm.Message, error) {
		const maxAttempts = 5
		backoff := time.Second
		for attempt := 0; attempt < maxAttempts; attempt++ {
			msg, err := srv.Users.Messages.Get(user, id).Format("full").Do()
			if err == nil {
				return msg, nil
			}
			if strings.Contains(err.Error(), "rateLimitExceeded") || strings.Contains(err.Error(), "backendError") {
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return nil, err
		}
		return nil, fmt.Errorf("max retries exceeded")
	}

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				msg, err := getMessage(id)
				if err != nil {
					// Log and continue; skip this message.
					log.Printf("get message %s: %v", id, err)
					bar.Add(1)
					continue
				}
				subject := getSubject(msg.Payload.Headers)

				// Parse outside the lock; only mutate maps inside lock.
				switch {
				case strings.Contains(subject, "Canceled:"):
					mu.Lock()
					processCanceledEmail(subject, orders)
					mu.Unlock()
				case strings.HasSuffix(subject, "was canceled 🔴"):
					mu.Lock()
					processPaymentFailCancelEmail(msg, orders)
					mu.Unlock()
				case strings.Contains(subject, "Shipped:"):
					newShipped := processShippedEmail(msg)
					if len(newShipped) > 0 {
						mu.Lock()
						for _, s := range newShipped {
							if _, ok := shippedIDs[s.TrackingNumber]; !ok {
								shipped = append(shipped, s)
								shippedIDs[s.TrackingNumber] = struct{}{}
							}
						}
						mu.Unlock()
					}
				case strings.HasPrefix(subject, "Arrived:"), strings.HasPrefix(subject, "Delivered:"):
					deliveredOrderID := processDeliveredEmail(msg)
					if deliveredOrderID != "" {
						mu.Lock()
						// Add to shipped list so it gets filtered out of live orders
						if _, ok := shippedIDs[deliveredOrderID]; !ok {
							shipped = append(shipped, &report.ShippedOrder{
								ID:               deliveredOrderID,
								TrackingNumber:   "DELIVERED",
								Carrier:          "Delivered",
								EstimatedArrival: "",
							})
							shippedIDs[deliveredOrderID] = struct{}{}
						}
						mu.Unlock()
					}
				default:
					docMsg := msg
					order := func() *report.Order {
						doc, err := parseMessageHTML(docMsg)
						if err != nil {
							return nil
						}
						return extractOrderInfo(doc, subject)
					}()
					if order != nil && order.ID != "" {
						mu.Lock()
						mergeOrCreateOrder(orders, order)
						mu.Unlock()
					}
				}

				bar.Add(1)
			}
		}()
	}

	for _, m := range allMessages {
		jobs <- m.Id
	}
	close(jobs)
	wg.Wait()

	return orders, shipped, nil
}
