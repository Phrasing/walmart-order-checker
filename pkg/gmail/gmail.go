package gmail

import (
	"context"
	"encoding/json"
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
	decoded, err := base64.URLEncoding.DecodeString(data)
	if err == nil {
		return string(decoded), nil
	}
	decoded, err = base64.StdEncoding.DecodeString(data)
	if err == nil {
		return string(decoded), nil
	}
	decoded, err = base64.RawURLEncoding.DecodeString(data)
	if err == nil {
		return string(decoded), nil
	}
	decoded, err = base64.RawStdEncoding.DecodeString(data)
	if err == nil {
		return string(decoded), nil
	}
	return "", err
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
	body := findHTMLPart(msg.Payload)
	if body == "" {
		log.Printf("Could not find HTML part for message %v", msg.Id)
		return
	}

	decodedBody, err := decodeBase64(body)
	if err != nil {
		log.Printf("Error decoding body for message %v: %v", msg.Id, err)
		return
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(decodedBody))
	if err != nil {
		log.Printf("Error parsing HTML for message %v: %v", msg.Id, err)
		return
	}

	orderID := strings.TrimSpace(doc.Find("a[aria-label*=' ']").First().Text())
	orderID = strings.ReplaceAll(orderID, "-", "")

	trackingNumber := doc.Find("span:contains('tracking number') a").Text()
	carrierAndTracking := doc.Find("span:contains('tracking number')").Text()
	re := regexp.MustCompile(`(\w+)\s+tracking\s+number`)
	matches := re.FindStringSubmatch(carrierAndTracking)
	var carrier string
	if len(matches) > 1 {
		carrier = matches[1]
	}
	estimatedArrival := doc.Find("strong:contains('Arrives')").Text()

	if _, ok := shippedOrders[orderID]; !ok {
		shippedOrders[orderID] = &report.ShippedOrder{
			ID:               orderID,
			TrackingNumber:   trackingNumber,
			Carrier:          carrier,
			EstimatedArrival: estimatedArrival,
		}
	}
}

func processOrderConfirmationEmail(msg *gmail.Message, subject string, orders map[string]*report.Order) {
	body := findHTMLPart(msg.Payload)
	if body == "" {
		log.Printf("Could not find HTML part for message %v", msg.Id)
		return
	}

	decodedBody, err := decodeBase64(body)
	if err != nil {
		log.Printf("Error decoding body for message %v: %v", msg.Id, err)
		return
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(decodedBody))
	if err != nil {
		log.Printf("Error parsing HTML for message %v: %v", msg.Id, err)
		return
	}

	orderID := strings.TrimSpace(doc.Find("a[aria-label*=' ']").First().Text())
	orderID = strings.ReplaceAll(orderID, "-", "")
	total := doc.Find("strong:contains('Includes all fees, taxes, discounts and driver tip')").Parent().Next().Find("strong").Text()

	var orderDate string
	var parsedDate time.Time
	dateText := doc.Find("div:contains('Order date:')").Text()
	if dateText != "" {
		re := regexp.MustCompile(`Order date:\s*(.*)`)
		matches := re.FindStringSubmatch(dateText)
		if len(matches) > 1 {
			orderDate = strings.TrimSpace(matches[1])
			var err error
			parsedDate, err = time.Parse("Mon, Jan 2, 2006", orderDate)
			if err != nil {
				log.Printf("Could not parse date for order %s: %v", orderID, err)
			}
		}
	}

	var items []report.Item
	doc.Find("img[alt*='quantity']").Each(func(i int, s *goquery.Selection) {
		altText := s.AttrOr("alt", "")
		imageURL := s.AttrOr("src", "")
		parts := strings.Split(altText, " item ")
		if len(parts) == 2 {
			qtyParts := strings.Split(parts[0], " ")
			qty := 1
			if len(qtyParts) > 1 {
				fmt.Sscanf(qtyParts[1], "%d", &qty)
			}
			items = append(items, report.Item{Name: parts[1], Quantity: qty, ImageURL: imageURL})
		}
	})

	status := "confirmed"
	if strings.Contains(subject, "preorder") {
		status = "pre-ordered"
	}

	if existingOrder, ok := orders[orderID]; ok {
		if len(existingOrder.Items) == 0 {
			existingOrder.Items = items
		}
		if existingOrder.Status != "canceled" {
			existingOrder.Status = status
		}
	} else {
		orders[orderID] = &report.Order{
			ID:              orderID,
			Items:           items,
			Total:           total,
			OrderDate:       orderDate,
			OrderDateParsed: parsedDate,
			Status:          status,
		}
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

	numWorkers := 10
	jobs := make(chan string, len(allMessages))
	var wg sync.WaitGroup

	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for msgID := range jobs {
				msg, err := srv.Users.Messages.Get(user, msgID).Format("full").Do()
				if err != nil {
					if strings.Contains(err.Error(), "rateLimitExceeded") {
						log.Printf("Rate limit exceeded for message %v. Retrying after 1 second.", msgID)
						time.Sleep(1 * time.Second)
						jobs <- msgID
					} else {
						log.Printf("Unable to retrieve message %v: %v", msgID, err)
					}
					continue
				}

				subject := ""
				for _, h := range msg.Payload.Headers {
					if h.Name == "Subject" {
						subject = h.Value
						break
					}
				}

				mu.Lock()
				if strings.Contains(subject, "Canceled") {
					processCanceledEmail(subject, orders)
				} else if strings.Contains(subject, "Shipped") {
					processShippedEmail(msg, shippedOrders)
				} else {
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
