package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"

	"walmart-order-checker/internal/api"
	"walmart-order-checker/internal/auth"
	"walmart-order-checker/internal/security"
	"walmart-order-checker/internal/storage"
)

func main() {
	godotenv.Load()

	port := flag.String("port", "3000", "Port to run the server on")
	flag.Parse()

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("REDIRECT_URL")

	if redirectURL == "" {
		redirectURL = fmt.Sprintf("http://localhost:%s/api/auth/callback", *port)
	}

	if clientID == "" || clientSecret == "" {
		log.Fatal("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET environment variables are required")
	}

	if err := security.CheckSecretFilesPermissions(".data"); err != nil {
		log.Printf("Warning: %v", err)
	}

	tokenStorage, err := storage.NewTokenStorage(".data/tokens.db")
	if err != nil {
		log.Fatalf("Failed to initialize token storage: %v", err)
	}
	defer tokenStorage.Close()

	if err := security.CheckSecretFilesPermissions(".data"); err != nil {
		log.Printf("Warning: Failed to verify file permissions after creating database: %v", err)
	}

	authManager := auth.NewManager(clientID, clientSecret, redirectURL, tokenStorage)
	server := api.NewServer(authManager, tokenStorage)

	globalRateLimiter := api.NewRateLimiter(100, 10)
	authRateLimiter := api.NewRateLimiter(20, 5)

	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(api.SecurityHeadersMiddleware)
	r.Use(globalRateLimiter.Middleware)

	frontendURL := os.Getenv("FRONTEND_URL")
	allowedOrigins := []string{
		"http://localhost:3000",
		"http://localhost:5173",
		"http://127.0.0.1:3000",
		"http://127.0.0.1:5173",
	}
	if frontendURL != "" {
		allowedOrigins = append(allowedOrigins, frontendURL)
	}

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Use(authRateLimiter.Middleware)
			r.Get("/login", server.HandleLogin)
			r.Get("/callback", server.HandleCallback)
			r.Post("/logout", server.HandleLogout)
			r.Get("/status", server.HandleAuthStatus)
		})

		r.Group(func(r chi.Router) {
			r.Use(api.JSONMiddleware)

			r.Post("/scan", server.HandleScan)
			r.Get("/scan/status", server.HandleScanStatus)
			r.Get("/report", server.HandleReport)

			r.Get("/cache/stats", server.HandleCacheStats)
			r.Delete("/cache/clear", server.HandleCacheClear)
		})

		r.Get("/ws/scan", server.HandleWebSocket(authManager))
	})

	fileServer := http.FileServer(http.Dir("./web/dist"))
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		if _, err := http.Dir("./web/dist").Open(r.URL.Path); err != nil {
			http.ServeFile(w, r, "./web/dist/index.html")
		} else {
			fileServer.ServeHTTP(w, r)
		}
	})

	addr := ":" + *port
	log.Printf("Starting server on http://localhost%s", addr)
	log.Printf("OAuth redirect URL: %s", redirectURL)
	log.Fatal(http.ListenAndServe(addr, r))
}
