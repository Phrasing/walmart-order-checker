package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const (
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorReset  = "\033[0m"
)

var daysToScan int

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

func getClient(config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Println("Attempting to open the authorization link in your browser.")
	fmt.Printf("If it doesn't open automatically, please go to this link:\n%v\n", authURL)

	err := openBrowser(authURL)
	if err != nil {
		log.Printf("Failed to open browser: %v. Please open the URL manually.", err)
	}

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

	authCode := <-codeChan

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Fatalf("Failed to shutdown server: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default:
		cmd = "xdg-open"
	}

	args = append(args, url)
	return exec.Command(cmd, args...).Start()
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

func main() {
	flag.IntVar(&daysToScan, "days", 10, "Number of days back to scan for emails.")
	flag.Parse()

	credentials, err := os.ReadFile("credentials.json")
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("\n%s*** Welcome to Walmart Order Checker ***%s\n\n", colorGreen, colorReset)
			fmt.Printf("%sAction Required: `credentials.json` not found.%s\n", colorRed, colorReset)
			fmt.Println("Please follow these steps to set up your credentials:")
			fmt.Println(colorYellow + "--------------------------------------------------------------------------------" + colorReset)
			fmt.Printf("%sStep 1: Enable the Gmail API%s\n", colorCyan, colorReset)
			fmt.Println("   - Visit: https://console.cloud.google.com/marketplace/product/google/gmail.googleapis.com")
			fmt.Println("   - Make sure you are logged into your Google account and have a project selected.")
			fmt.Println()
			fmt.Printf("%sStep 2: Create OAuth Credentials%s\n", colorCyan, colorReset)
			fmt.Println("   - Visit: https://console.cloud.google.com/auth/clients/create")
			fmt.Println("   - Select 'Desktop app' as the application type.")
			fmt.Println("   - Give it a name, like 'Walmart Order Checker Client'.")
			fmt.Println("   - Click 'Create'.")
			fmt.Println()
			fmt.Printf("%sStep 3: Configure Consent Screen%s\n", colorCyan, colorReset)
			fmt.Println("   - You may be prompted to configure the consent screen.")
			fmt.Println("   - Choose 'External' and provide an app name, support email, and contact email.")
			fmt.Println("   - Add your email address as a 'Test user'.")
			fmt.Println()
			fmt.Printf("%sStep 4: Download and Save Credentials%s\n", colorCyan, colorReset)
			fmt.Println("   - Download the credentials file from the list.")
			fmt.Println("   - Rename it to `credentials.json` and place it in the same folder as this tool.")
			fmt.Println(colorYellow + "--------------------------------------------------------------------------------" + colorReset)
			fmt.Printf("%sOnce you've done this, please run the application again.%s\n\n", colorGreen, colorReset)
			return
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

	user := "me"
	profile, err := srv.Users.GetProfile(user).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve user profile: %v", err)
	}

	query := fmt.Sprintf("from:help@walmart.com subject:(\"thanks for your preorder\" OR \"thanks for your order\" OR \"Canceled: delivery from order\" OR \"Shipped:\") newer_than:%dd", daysToScan)

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

	orders := make(map[string]*Order)
	shippedOrders := make(map[string]*ShippedOrder)
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

				var orderID string
				var items []Item
				var status string
				var total string
				var orderDate string
				var parsedDate time.Time
				var trackingNumber string
				var carrier string
				var estimatedArrival string

				subject := ""
				for _, h := range msg.Payload.Headers {
					if h.Name == "Subject" {
						subject = h.Value
						break
					}
				}

				if strings.Contains(subject, "Canceled") {
					status = "canceled"
					parts := strings.Split(subject, "#")
					if len(parts) > 1 {
						orderID = parts[1]
					}
				} else if strings.Contains(subject, "Shipped") {
					status = "shipped"
					body := findHTMLPart(msg.Payload)
					if body == "" {
						log.Printf("Could not find HTML part for message %v", msgID)
						continue
					}

					decodedBody, err := decodeBase64(body)
					if err != nil {
						log.Printf("Error decoding body for message %v: %v", msgID, err)
						continue
					}

					doc, err := goquery.NewDocumentFromReader(strings.NewReader(decodedBody))
					if err != nil {
						log.Printf("Error parsing HTML for message %v: %v", msgID, err)
						continue
					}

					orderID = strings.TrimSpace(doc.Find("a[aria-label*=' ']").First().Text())
					orderID = strings.ReplaceAll(orderID, "-", "")

					doc.Find("span:contains('tracking number')").Each(func(i int, s *goquery.Selection) {
						trackingNumber = s.Find("a").Text()
						carrierAndTracking := s.Text()
						re := regexp.MustCompile(`(\w+)\s+tracking\s+number`)
						matches := re.FindStringSubmatch(carrierAndTracking)
						if len(matches) > 1 {
							carrier = matches[1]
						}
					})

					doc.Find("strong:contains('Arrives')").Each(func(i int, s *goquery.Selection) {
						estimatedArrival = s.Text()
					})

				} else {
					if strings.Contains(subject, "preorder") {
						status = "pre-ordered"
					} else {
						status = "confirmed"
					}

					body := findHTMLPart(msg.Payload)
					if body == "" {
						log.Printf("Could not find HTML part for message %v", msgID)
						continue
					}

					decodedBody, err := decodeBase64(body)
					if err != nil {
						log.Printf("Error decoding body for message %v: %v", msgID, err)
						continue
					}

					doc, err := goquery.NewDocumentFromReader(strings.NewReader(decodedBody))
					if err != nil {
						log.Printf("Error parsing HTML for message %v: %v", msgID, err)
						continue
					}

					orderID = strings.TrimSpace(doc.Find("a[aria-label*=' ']").First().Text())
					orderID = strings.ReplaceAll(orderID, "-", "")

					doc.Find("strong:contains('Includes all fees, taxes, discounts and driver tip')").Each(func(i int, s *goquery.Selection) {
						total = s.Parent().Next().Find("strong").Text()
					})

					doc.Find("div:contains('Order date:')").Each(func(i int, s *goquery.Selection) {
						dateText := s.Text()
						if after, ok := strings.CutPrefix(dateText, " Order date:"); ok {
							orderDate = strings.TrimSpace(after)
							var err error
							parsedDate, err = time.Parse("Mon, Jan 2, 2006", orderDate)
							if err != nil {
								log.Printf("Could not parse date for order %s: %v", orderID, err)
							}
						}
					})

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
							items = append(items, Item{Name: parts[1], Quantity: qty, ImageURL: imageURL})
						}
					})
				}

				if orderID != "" {
					mu.Lock()
					if status == "shipped" {
						if _, ok := shippedOrders[orderID]; !ok {
							shippedOrders[orderID] = &ShippedOrder{
								ID:               orderID,
								TrackingNumber:   trackingNumber,
								Carrier:          carrier,
								EstimatedArrival: estimatedArrival,
							}
						}
					} else {
						if existingOrder, ok := orders[orderID]; ok {
							if status == "canceled" {
								existingOrder.Status = "canceled"
							} else {
								if len(existingOrder.Items) == 0 {
									existingOrder.Items = items
								}
								if existingOrder.Status != "canceled" {
									existingOrder.Status = status
								}
							}
						} else {
							orders[orderID] = &Order{
								ID:              orderID,
								Items:           items,
								Total:           total,
								OrderDate:       orderDate,
								OrderDateParsed: parsedDate,
								Status:          status,
							}
						}
					}
					mu.Unlock()
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

	var shippedOrdersSlice []*ShippedOrder
	for _, so := range shippedOrders {
		shippedOrdersSlice = append(shippedOrdersSlice, so)
	}

	generateHTML(orders, len(allMessages), daysToScan, htmlPath, shippedOrdersSlice)
	generateCSV(orders, csvPath)
	generateShippedCSV(shippedOrdersSlice, shippedCsvPath)

	fmt.Printf("HTML report has been generated: %s\n", htmlPath)
	openBrowser(htmlPath)
}

func formatOrderID(id string) string {
	if len(id) > 7 && !strings.Contains(id, "-") {
		return id[:7] + "-" + id[7:]
	}
	return id
}

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
