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

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

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

func InitializeGmailService() *gmail.Service {
	credentials, err := os.ReadFile("credentials.json")
	if err != nil {
		if os.IsNotExist(err) {
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
