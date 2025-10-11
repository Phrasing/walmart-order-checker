package gmail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sync"

	"encoding/base64"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"regexp"
	"time"

	"walmart-order-checker/pkg/report"

	"github.com/PuerkitoBio/goquery"
	"github.com/schollz/progressbar/v3"
)

func findHTMLPart(part *gmail.MessagePart) string {
	if part.MimeType == "text/html" && part.Body != nil && part.Body.Size > 0 {
		return part.Body.Data
	}

	if strings.HasPrefix(part.MimeType, "multipart/") {
		for _, subPart := range part.Parts {
			if htmlPart := findHTMLPart(subPart); htmlPart != "" {
				return htmlPart
			}
		}
	}
	return ""
}

func decodeBase64(data string) (string, error) {
	encodings := []base64.Encoding{
		*base64.URLEncoding,
		*base64.StdEncoding,
		*base64.RawURLEncoding,
		*base64.RawStdEncoding,
	}

	for _, enc := range encodings {
		if decoded, err := enc.DecodeString(data); err == nil {
			return string(decoded), nil
		}
	}

	return "", errors.New("failed to decode base64 with any encoding")
}

func getClient(config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func startOAuthWebServer(authURL string) string {
	codeChan := make(chan string)
	var once sync.Once

	srv := &http.Server{Addr: ":80"}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code != "" {
			once.Do(func() {
				fmt.Fprintf(w, "Authorization successful! You can close this window.")
				codeChan <- code
			})
		}
	})

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	err := OpenBrowser(authURL)
	if err != nil {
		log.Printf("Failed to open browser: %v. Please open the URL manually.", err)
	}

	authCode := <-codeChan

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Fatalf("Failed to shutdown server: %v", err)
	}
	return authCode
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Println("Attempting to open the authorization link in your browser.")
	fmt.Printf("If it doesn't open automatically, please go to this link:\n%v\n", authURL)

	authCode := startOAuthWebServer(authURL)

	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

func OpenBrowser(url string) error {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	return err
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func handleMissingCredentials() {
	fmt.Printf("\n%s*** Welcome to Walmart Order Checker ***%s\n\n", "\033[32m", "\033[0m")
	fmt.Printf("%sAction Required: `credentials.json` not found.%s\n", "\033[31m", "\033[0m")
	fmt.Println("Please follow these steps to set up your credentials:")
	fmt.Println("\033[33m" + "--------------------------------------------------------------------------------" + "\033[0m")
	fmt.Printf("%sStep 1: Enable the Gmail API%s\n", "\033[36m", "\033[0m")
	fmt.Println("   - Visit: https://console.cloud.google.com/marketplace/product/google/gmail.googleapis.com")
	fmt.Println("   - Make sure you are logged into your Google account and have a project selected.")
	fmt.Println()
	fmt.Printf("%sStep 2: Create OAuth Credentials%s\n", "\033[36m", "\033[0m")
	fmt.Println("   - Visit: https://console.cloud.google.com/auth/clients/create")
	fmt.Println("   - Select 'Desktop app' as the application type.")
	fmt.Println("   - Give it a name, like 'Walmart Order Checker Client'.")
	fmt.Println("   - Click 'Create'.")
	fmt.Println()
	fmt.Printf("%sStep 3: Configure Consent Screen%s\n", "\033[36m", "\033[0m")
	fmt.Println("   - You may be prompted to configure the consent screen.")
	fmt.Println("   - Choose 'External' and provide an app name, support email, and contact email.")
	fmt.Println("   - Add your email address as a 'Test user'.")
	fmt.Println()
	fmt.Printf("%sStep 4: Download and Save Credentials%s\n", "\033[36m", "\033[0m")
	fmt.Println("   - Download the credentials file from the list.")
	fmt.Println("   - Rename it to `credentials.json` and place it in the same folder as this tool.")
	fmt.Println("\033[33m" + "--------------------------------------------------------------------------------" + "\033[0m")
	fmt.Printf("%sOnce you've done this, please run the application again.%s\n\n", "\033[32m", "\033[0m")
	os.Exit(1)
}

func InitializeGmailService() *gmail.Service {
	credentials, err := os.ReadFile("credentials.json")
	if err != nil {
		if os.IsNotExist(err) {
			handleMissingCredentials()
		}
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON(credentials, gmail.GmailReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := gmail.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Gmail client: %v", err)
	}
	return srv
}

func FetchMessages(srv *gmail.Service, user string, query string) []*gmail.Message {
	var allMessages []*gmail.Message
	pageToken := ""
	for {
		req := srv.Users.Messages.List(user).Q(query)
		if pageToken != "" {
			req.PageToken(pageToken)
		}
		r, err := req.Do()
		if err != nil {
			log.Fatalf("Unable to retrieve messages: %v", err)
		}

		allMessages = append(allMessages, r.Messages...)

		if r.NextPageToken == "" {
			break
		}
		pageToken = r.NextPageToken
	}
	return allMessages
}

func processCanceledEmail(subject string, orders map[string]*report.Order) {
	parts := strings.Split(subject, "#")
	if len(parts) > 1 {
		orderID := parts[1]
		if existingOrder, ok := orders[orderID]; ok {
			existingOrder.Status = "canceled"
		} else {
			orders[orderID] = &report.Order{ID: orderID, Status: "canceled"}
		}
	}
}

func processShippedEmail(msg *gmail.Message, shippedOrders map[string]*report.ShippedOrder) {
	doc, err := parseMessageHTML(msg)
	if err != nil {
		log.Printf("Error processing message %v: %v", msg.Id, err)
		return
	}

	order := extractShippingInfo(doc)
	if order.ID == "" {
		log.Printf("Could not extract order ID from message %v", msg.Id)
		return
	}

	if _, exists := shippedOrders[order.ID]; !exists {
		shippedOrders[order.ID] = order
	}
}

func parseMessageHTML(msg *gmail.Message) (*goquery.Document, error) {
	body := findHTMLPart(msg.Payload)
	if body == "" {
		return nil, fmt.Errorf("HTML part not found")
	}

	decodedBody, err := decodeBase64(body)
	if err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	return goquery.NewDocumentFromReader(strings.NewReader(decodedBody))
}

func extractShippingInfo(doc *goquery.Document) *report.ShippedOrder {
	orderID := strings.ReplaceAll(
		strings.TrimSpace(doc.Find("a[aria-label*=' ']").First().Text()),
		"-", "",
	)

	carrier := extractCarrier(doc)

	return &report.ShippedOrder{
		ID:               orderID,
		TrackingNumber:   doc.Find("span:contains('tracking number') a").Text(),
		Carrier:          carrier,
		EstimatedArrival: doc.Find("strong:contains('Arrives')").Text(),
	}
}

func extractCarrier(doc *goquery.Document) string {
	carrierText := doc.Find("span:contains('tracking number')").Text()
	re := regexp.MustCompile(`(\w+)\s+tracking\s+number`)
	if matches := re.FindStringSubmatch(carrierText); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func processOrderConfirmationEmail(msg *gmail.Message, subject string, orders map[string]*report.Order) {
	doc, err := parseMessageHTML(msg)
	if err != nil {
		log.Printf("Error processing message %v: %v", msg.Id, err)
		return
	}

	order := extractOrderInfo(doc, subject)
	if order.ID == "" {
		log.Printf("Could not extract order ID from message %v", msg.Id)
		return
	}

	mergeOrCreateOrder(orders, order)
}

func extractOrderInfo(doc *goquery.Document, subject string) *report.Order {
	orderID := strings.ReplaceAll(
		strings.TrimSpace(doc.Find("a[aria-label*=' ']").First().Text()),
		"-", "",
	)

	orderDate, parsedDate := extractOrderDate(doc, orderID)

	return &report.Order{
		ID:              orderID,
		Items:           extractItems(doc),
		Total:           extractTotal(doc),
		OrderDate:       orderDate,
		OrderDateParsed: parsedDate,
		Status:          determineStatus(subject),
	}
}

func extractOrderDate(doc *goquery.Document, orderID string) (string, time.Time) {
	dateText := doc.Find("div:contains('Order date:')").Text()
	re := regexp.MustCompile(`Order date:\s*(.*)`)
	matches := re.FindStringSubmatch(dateText)

	if len(matches) <= 1 {
		return "", time.Time{}
	}

	orderDate := strings.TrimSpace(matches[1])
	parsedDate, err := time.Parse("Mon, Jan 2, 2006", orderDate)
	if err != nil {
		log.Printf("Could not parse date for order %s: %v", orderID, err)
		return orderDate, time.Time{}
	}

	return orderDate, parsedDate
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
	altText := s.AttrOr("alt", "")
	parts := strings.Split(altText, " item ")

	if len(parts) != 2 {
		return report.Item{}, false
	}

	qty := 1
	qtyParts := strings.Split(parts[0], " ")
	if len(qtyParts) > 1 {
		fmt.Sscanf(qtyParts[1], "%d", &qty)
	}

	return report.Item{
		Name:     parts[1],
		Quantity: qty,
		ImageURL: s.AttrOr("src", ""),
	}, true
}

func determineStatus(subject string) string {
	if strings.Contains(subject, "preorder") {
		return "pre-ordered"
	}
	return "confirmed"
}

func mergeOrCreateOrder(orders map[string]*report.Order, newOrder *report.Order) {
	existing, exists := orders[newOrder.ID]
	if !exists {
		orders[newOrder.ID] = newOrder
		return
	}

	if len(existing.Items) == 0 {
		existing.Items = newOrder.Items
	}
	if existing.Status != "canceled" {
		existing.Status = newOrder.Status
	}
}

func ProcessEmails(srv *gmail.Service, user string, allMessages []*gmail.Message) (map[string]*report.Order, map[string]*report.ShippedOrder) {
	orders := make(map[string]*report.Order)
	shippedOrders := make(map[string]*report.ShippedOrder)
	var mu sync.Mutex

	bar := progressbar.NewOptions(len(allMessages),
		progressbar.OptionSetDescription("Processing emails"),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[cyan]█[reset]",
			SaucerHead:    "[blue]█[reset]",
			SaucerPadding: " ",
			BarStart:      "|",
			BarEnd:        "|",
		}))

	jobs := make(chan string, len(allMessages))
	var wg sync.WaitGroup

	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for msgID := range jobs {
				msg, err := srv.Users.Messages.Get(user, msgID).Format("full").Do()
				if err != nil {
					if strings.Contains(err.Error(), "rateLimitExceeded") {
						log.Printf("Rate limit exceeded for message %v. Retrying after 1 second.", msgID)
						time.Sleep(time.Second)
						jobs <- msgID
					} else {
						log.Printf("Unable to retrieve message %v: %v", msgID, err)
					}
					continue
				}

				subject := getSubject(msg.Payload.Headers)

				mu.Lock()
				switch {
				case strings.Contains(subject, "Canceled"):
					processCanceledEmail(subject, orders)
				case strings.Contains(subject, "Shipped"):
					processShippedEmail(msg, shippedOrders)
				default:
					processOrderConfirmationEmail(msg, subject, orders)
				}
				mu.Unlock()
				bar.Add(1)
			}
		}()
	}

	for _, m := range allMessages {
		jobs <- m.Id
	}
	close(jobs)

	wg.Wait()
	return orders, shippedOrders
}

func getSubject(headers []*gmail.MessagePartHeader) string {
	for _, h := range headers {
		if h.Name == "Subject" {
			return h.Value
		}
	}
	return ""
}
