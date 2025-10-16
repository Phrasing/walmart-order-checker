package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	gm "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"walmart-order-checker/internal/security"
	"walmart-order-checker/internal/storage"
)

const (
	sessionName   = "walmart-checker-session"
	oauthStateKey = "oauth-state"
	emailKey      = "user-email"
)

type Manager struct {
	config       *oauth2.Config
	store        *sessions.CookieStore
	tokenStorage *storage.TokenStorage
}

func NewManager(clientID, clientSecret, redirectURL string, tokenStorage *storage.TokenStorage) *Manager {
	sessionKey := os.Getenv("SESSION_KEY")
	if sessionKey == "" {
		environment := os.Getenv("ENVIRONMENT")
		if environment == "production" {
			log.Fatal("SESSION_KEY environment variable is required in production")
		}

		log.Println("WARNING: SESSION_KEY not set, generating temporary key (development only)")
		log.Println("WARNING: All sessions will be invalidated on restart!")
		var err error
		sessionKey, err = security.GenerateSessionKey()
		if err != nil {
			log.Fatalf("Failed to generate session key: %v", err)
		}
	}

	sessionKeyBytes, err := security.DecodeKey(sessionKey)
	if err != nil {
		log.Fatalf("Invalid SESSION_KEY: %v", err)
	}

	return &Manager{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{gmail.GmailReadonlyScope},
			Endpoint:     google.Endpoint,
		},
		store:        sessions.NewCookieStore(sessionKeyBytes),
		tokenStorage: tokenStorage,
	}
}

func generateRandomState() (string, error) {
	return security.GenerateSessionKey()
}

func (m *Manager) GetLoginURL(w http.ResponseWriter, r *http.Request) (string, error) {
	state, err := generateRandomState()
	if err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}

	session, _ := m.store.Get(r, sessionName)
	session.Values[oauthStateKey] = state
	session.Options = getSessionOptionsForOAuth(r, 300)

	if err := session.Save(r, w); err != nil {
		return "", fmt.Errorf("save session: %w", err)
	}

	url := m.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	return url, nil
}

func getSessionOptionsForOAuth(r *http.Request, maxAge int) *sessions.Options {
	secure := isSecureContext(r)

	return &sessions.Options{
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func getSecureSessionOptions(r *http.Request, maxAge int) *sessions.Options {
	secure := isSecureContext(r)

	return &sessions.Options{
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	}
}

func isSecureContext(r *http.Request) bool {
	environment := os.Getenv("ENVIRONMENT")
	if environment == "production" {
		return true
	}

	if r.TLS != nil {
		return true
	}

	if r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}

	return false
}

func (m *Manager) HandleCallback(w http.ResponseWriter, r *http.Request) error {
	session, _ := m.store.Get(r, sessionName)

	storedState, ok := session.Values[oauthStateKey].(string)
	if !ok || storedState == "" {
		return fmt.Errorf("missing state in session")
	}

	state := r.URL.Query().Get("state")
	if state != storedState {
		return fmt.Errorf("invalid state parameter")
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		return fmt.Errorf("missing code parameter")
	}

	token, err := m.config.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}

	client := m.config.Client(context.Background(), token)
	srv, err := gm.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("create gmail service: %w", err)
	}

	profile, err := srv.Users.GetProfile("me").Do()
	if err != nil {
		return fmt.Errorf("get profile: %w", err)
	}

	if err := m.tokenStorage.Save(profile.EmailAddress, token); err != nil {
		return fmt.Errorf("save token: %w", err)
	}

	session.Values[emailKey] = profile.EmailAddress
	delete(session.Values, oauthStateKey)
	session.Options = getSecureSessionOptions(r, 86400*7)

	return session.Save(r, w)
}

func (m *Manager) GetToken(r *http.Request) (*oauth2.Token, string, error) {
	session, _ := m.store.Get(r, sessionName)

	email, ok := session.Values[emailKey].(string)
	if !ok || email == "" {
		return nil, "", fmt.Errorf("no user in session")
	}

	token, err := m.tokenStorage.Load(email)
	if err != nil {
		return nil, "", fmt.Errorf("load token: %w", err)
	}

	if token.Expiry.Before(time.Now()) {
		newToken, err := m.config.TokenSource(context.Background(), token).Token()
		if err != nil {
			return nil, "", fmt.Errorf("refresh token: %w", err)
		}

		if err := m.tokenStorage.Save(email, newToken); err != nil {
			return nil, "", fmt.Errorf("save refreshed token: %w", err)
		}

		return newToken, email, nil
	}

	return token, email, nil
}

func (m *Manager) IsAuthenticated(r *http.Request) bool {
	_, _, err := m.GetToken(r)
	return err == nil
}

func (m *Manager) Logout(w http.ResponseWriter, r *http.Request) error {
	session, _ := m.store.Get(r, sessionName)
	session.Options.MaxAge = -1
	return session.Save(r, w)
}

func (m *Manager) GetGmailService(r *http.Request) (*gm.Service, string, error) {
	token, email, err := m.GetToken(r)
	if err != nil {
		return nil, "", err
	}

	client := m.config.Client(context.Background(), token)
	srv, err := gm.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, "", fmt.Errorf("create gmail service: %w", err)
	}

	return srv, email, nil
}
