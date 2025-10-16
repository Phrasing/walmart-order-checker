package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"walmart-order-checker/internal/auth"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkWebSocketOrigin,
}

func checkWebSocketOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}

	allowedOriginsStr := os.Getenv("ALLOWED_WS_ORIGINS")
	if allowedOriginsStr == "" {
		allowedOriginsStr = "http://localhost:3000,http://localhost:5173,http://127.0.0.1:3000,http://127.0.0.1:5173"
	}

	allowedOrigins := strings.Split(allowedOriginsStr, ",")
	for _, allowed := range allowedOrigins {
		allowed = strings.TrimSpace(allowed)
		if origin == allowed {
			return true
		}
	}

	log.Printf("WebSocket: Rejected connection from unauthorized origin: %s", origin)
	return false
}

func (s *Server) HandleWebSocket(authManager *auth.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authManager.IsAuthenticated(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Set up ping/pong to keep connection alive
		const (
			writeWait      = 10 * time.Second
			pongWait       = 60 * time.Second
			pingPeriod     = (pongWait * 9) / 10   // Send pings at 90% of pong wait time
			updateInterval = 50 * time.Millisecond // Update every 50ms for responsive UI
		)

		// Set pong handler
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		// Set initial read deadline
		conn.SetReadDeadline(time.Now().Add(pongWait))

		// Channel to signal when read goroutine exits
		done := make(chan struct{})

		// Start goroutine to read and discard messages (to process pongs)
		go func() {
			defer close(done)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					break
				}
			}
		}()

		updateTicker := time.NewTicker(updateInterval)
		defer updateTicker.Stop()

		pingTicker := time.NewTicker(pingPeriod)
		defer pingTicker.Stop()

		for {
			select {
			case <-done:
				// Connection closed by client or read error
				return

			case <-updateTicker.C:
				s.scanMu.Lock()
				if s.activeScan != nil {
					data, err := json.Marshal(s.activeScan)
					s.scanMu.Unlock()

					if err != nil {
						log.Printf("JSON marshal error: %v", err)
						continue
					}

					conn.SetWriteDeadline(time.Now().Add(writeWait))
					if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
						return
					}
				} else {
					s.scanMu.Unlock()
				}

			case <-pingTicker.C:
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}
}
