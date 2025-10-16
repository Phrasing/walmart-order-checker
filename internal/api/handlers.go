package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	gm "google.golang.org/api/gmail/v1"

	"walmart-order-checker/internal/auth"
	"walmart-order-checker/internal/storage"
	"walmart-order-checker/pkg/gmail"
	"walmart-order-checker/pkg/report"
)

type Server struct {
	authManager  *auth.Manager
	tokenStorage *storage.TokenStorage
	scanMu       sync.Mutex
	activeScan   *ScanProgress
}

type ScanProgress struct {
	InProgress         bool                     `json:"in_progress"`
	TotalMessages      int                      `json:"total_messages"`
	Processed          int                      `json:"processed"`
	CurrentEmail       string                   `json:"current_email"`
	StartTime          time.Time                `json:"start_time"`
	LastProgressUpdate time.Time                `json:"-"` // Internal field, not sent to frontend
	Orders             map[string]*report.Order `json:"orders,omitempty"`
	Shipped            []*report.ShippedOrder   `json:"shipped,omitempty"`
	Error              string                   `json:"error,omitempty"`
	DaysScanned        int                      `json:"days_scanned,omitempty"`
}

func NewServer(authManager *auth.Manager, tokenStorage *storage.TokenStorage) *Server {
	return &Server{
		authManager:  authManager,
		tokenStorage: tokenStorage,
	}
}

func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	url, err := s.authManager.GetLoginURL(w, r)
	if err != nil {
		http.Error(w, "Failed to generate login URL", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"url": url,
	})
}

func (s *Server) HandleCallback(w http.ResponseWriter, r *http.Request) {
	if err := s.authManager.HandleCallback(w, r); err != nil {
		log.Printf("OAuth callback error: %v", err)
		http.Redirect(w, r, "/?error=auth_failed", http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func (s *Server) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.authManager.Logout(w, r); err != nil {
		http.Error(w, "Failed to logout", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "logged_out",
	})
}

func (s *Server) HandleAuthStatus(w http.ResponseWriter, r *http.Request) {
	authenticated := s.authManager.IsAuthenticated(r)

	response := map[string]interface{}{
		"authenticated": authenticated,
	}

	if authenticated {
		_, email, _ := s.authManager.GetToken(r)
		response["email"] = email
	}

	json.NewEncoder(w).Encode(response)
}

func (s *Server) HandleScan(w http.ResponseWriter, r *http.Request) {
	if !s.authManager.IsAuthenticated(r) {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	s.scanMu.Lock()
	if s.activeScan != nil && s.activeScan.InProgress {
		s.scanMu.Unlock()
		http.Error(w, "Scan already in progress", http.StatusConflict)
		return
	}
	s.scanMu.Unlock()

	var req struct {
		Days       int  `json:"days"`
		ClearCache bool `json:"clear_cache"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Days <= 0 {
		req.Days = 10
	}
	if req.Days > 365 {
		req.Days = 365
	}

	srv, email, err := s.authManager.GetGmailService(r)
	if err != nil {
		http.Error(w, "Failed to get Gmail service", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	s.scanMu.Lock()
	s.activeScan = &ScanProgress{
		InProgress:         true,
		StartTime:          now,
		LastProgressUpdate: now,
		CurrentEmail:       email,
		DaysScanned:        req.Days,
	}
	s.scanMu.Unlock()

	// Create cancellable context for timeout detection
	ctx, cancel := context.WithCancel(context.Background())
	go s.watchProgress(cancel)
	go s.runScan(ctx, srv, email, req.Days, req.ClearCache)

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "scan_started",
	})
}

func (s *Server) watchProgress(cancel context.CancelFunc) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.scanMu.Lock()
		if s.activeScan == nil || !s.activeScan.InProgress {
			s.scanMu.Unlock()
			return // Scan completed normally
		}

		idle := time.Since(s.activeScan.LastProgressUpdate)
		if idle > 30*time.Second {
			log.Printf("Scan timeout: no progress for %v (processed: %d/%d)", idle, s.activeScan.Processed, s.activeScan.TotalMessages)
			s.activeScan.Error = "Scan timed out - no progress for 30 seconds. Please try again."
			s.activeScan.InProgress = false
			s.scanMu.Unlock()
			cancel() // Stop all workers
			return
		}
		s.scanMu.Unlock()
	}
}

func (s *Server) runScan(ctx context.Context, srv interface{}, email string, days int, clearCache bool) {
	log.Printf("Scan started: %d days for %s", days, email)
	defer func() {
		s.scanMu.Lock()
		if s.activeScan != nil {
			s.activeScan.InProgress = false
		}
		s.scanMu.Unlock()
	}()

	gmailSrv, ok := srv.(*gm.Service)
	if !ok {
		s.scanMu.Lock()
		if s.activeScan != nil {
			s.activeScan.Error = "invalid gmail service"
		}
		s.scanMu.Unlock()
		return
	}

	if clearCache {
		log.Printf("Clearing cache...")
		clearStart := time.Now()
		cache := gmail.NewMessageCache(".cache/messages", 24*time.Hour)
		cache.Clear()
		log.Printf("Cache cleared in %v", time.Since(clearStart))
	}

	query := buildQuery(days)
	messages, err := gmail.FetchMessages(gmailSrv, "me", query)
	if err != nil {
		s.scanMu.Lock()
		if s.activeScan != nil {
			s.activeScan.Error = err.Error()
		}
		s.scanMu.Unlock()
		log.Printf("Scan failed: %v", err)
		return
	}

	log.Printf("Processing %d messages...", len(messages))
	s.scanMu.Lock()
	if s.activeScan != nil {
		s.activeScan.TotalMessages = len(messages)
	}
	s.scanMu.Unlock()

	// Create progress callback to update scan progress and last update time
	progressCallback := func(processed int) {
		s.scanMu.Lock()
		if s.activeScan != nil && s.activeScan.Processed != processed {
			s.activeScan.Processed = processed
			s.activeScan.LastProgressUpdate = time.Now()
		}
		s.scanMu.Unlock()
	}

	orders, shipped, err := gmail.ProcessEmailsWithProgress(ctx, gmailSrv, "me", messages, progressCallback)
	if err != nil {
		s.scanMu.Lock()
		if s.activeScan != nil {
			s.activeScan.Error = err.Error()
		}
		s.scanMu.Unlock()
		log.Printf("Scan failed: %v", err)
		return
	}

	s.scanMu.Lock()
	if s.activeScan != nil {
		s.activeScan.Orders = orders
		s.activeScan.Shipped = shipped
		s.activeScan.Processed = len(messages)
	}
	s.scanMu.Unlock()

	log.Printf("Scan completed: %d orders, %d shipments", len(orders), len(shipped))
}

func buildQuery(days int) string {
	return "from:help@walmart.com subject:(\"thanks for your preorder\" OR \"thanks for your order\" OR \"Canceled: delivery from order\" OR \"was canceled\" OR \"Shipped:\" OR \"Arrived:\" OR \"Delivered:\") newer_than:" + strconv.Itoa(days) + "d"
}

func (s *Server) HandleScanStatus(w http.ResponseWriter, r *http.Request) {
	if !s.authManager.IsAuthenticated(r) {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	if s.activeScan == nil {
		json.NewEncoder(w).Encode(map[string]bool{
			"in_progress": false,
		})
		return
	}

	json.NewEncoder(w).Encode(s.activeScan)
}

func (s *Server) HandleReport(w http.ResponseWriter, r *http.Request) {
	if !s.authManager.IsAuthenticated(r) {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	if s.activeScan == nil || s.activeScan.Orders == nil {
		http.Error(w, "No scan results available", http.StatusNotFound)
		return
	}

	orders := s.activeScan.Orders
	shipped := s.activeScan.Shipped

	nonCanceled := filterNonCanceled(orders)
	learned := report.LearnPrices(nonCanceled)
	productSummaries := buildProductSummaries(nonCanceled, learned)
	orderDetails := report.PrepareOrderDetails(nonCanceled, learned)
	liveOrdersFiltered := filterLiveOrders(nonCanceled, shipped)
	liveOrdersForTemplate := report.PrepareOrderDetails(liveOrdersFiltered, learned)
	liveOrderSummary := report.CalculateLiveOrderSummary(liveOrdersFiltered, learned)
	emailStats := report.CalculateEmailStats(orders, len(liveOrdersFiltered))
	productCancel := report.CalculateProductStats(orders)

	daysScanned := s.activeScan.DaysScanned
	if daysScanned == 0 {
		daysScanned = 10
	}

	response := map[string]interface{}{
		"orders":             orders,
		"shipped":            shipped,
		"email_stats":        emailStats,
		"live_order_summary": liveOrderSummary,
		"live_orders":        liveOrdersForTemplate,
		"product_cancel":     productCancel,
		"order_lines":        orderDetails,
		"product_spend":      productSummaries,
		"shipments":          shipped,
		"date_range":         buildDateRange(daysScanned),
	}

	json.NewEncoder(w).Encode(response)
}

func filterNonCanceled(orders map[string]*report.Order) []*report.Order {
	var result []*report.Order
	for _, order := range orders {
		if order.Status != "canceled" {
			result = append(result, order)
		}
	}
	return result
}

func filterLiveOrders(nonCanceledOrders []*report.Order, shippedOrders []*report.ShippedOrder) []*report.Order {
	shippedIDs := make(map[string]struct{})
	for _, s := range shippedOrders {
		shippedIDs[s.ID] = struct{}{}
	}

	var liveOrders []*report.Order
	for _, o := range nonCanceledOrders {
		if _, isShipped := shippedIDs[o.ID]; !isShipped {
			liveOrders = append(liveOrders, o)
		}
	}
	return liveOrders
}

func buildProductSummaries(orders []*report.Order, learnedPrices map[string]float64) []report.ProductSummary {
	m := report.CalculateSummaries(orders, learnedPrices)
	out := make([]report.ProductSummary, 0, len(m))
	for _, s := range m {
		out = append(out, *s)
	}
	return out
}

func buildDateRange(days int) string {
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -days)
	return "Email Scan Range: " + startDate.Format("Jan 2, 2006") + " to " + endDate.Format("Jan 2, 2006") + " (" + strconv.Itoa(days) + " days)"
}

func (s *Server) HandleCacheStats(w http.ResponseWriter, r *http.Request) {
	cache := gmail.NewMessageCache(".cache/messages", 24*time.Hour)
	total, size, err := cache.Stats()
	if err != nil {
		http.Error(w, "Failed to get cache stats", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_messages": total,
		"total_size":     size,
	})
}

func (s *Server) HandleCacheClear(w http.ResponseWriter, r *http.Request) {
	if !s.authManager.IsAuthenticated(r) {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	cache := gmail.NewMessageCache(".cache/messages", 24*time.Hour)
	if err := cache.Clear(); err != nil {
		http.Error(w, "Failed to clear cache", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "cache_cleared",
	})
}
