package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // CORS handled by middleware
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

type wsHub struct {
	clients    map[*wsClient]bool
	broadcast  chan []byte
	register   chan *wsClient
	unregister chan *wsClient
	mu         sync.RWMutex
}

func newHub() *wsHub {
	return &wsHub{
		clients:    make(map[*wsClient]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *wsClient),
		unregister: make(chan *wsClient),
	}
}

func (h *wsHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &wsClient{
		conn: conn,
		send: make(chan []byte, 64),
	}

	// Writer goroutine
	go func() {
		defer conn.Close()
		for msg := range client.send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	// Push real-time metrics every second
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		defer conn.Close()

		for {
			select {
			case <-ticker.C:
				data := s.collectRealtimeMetrics()
				msg, _ := json.Marshal(data)
				select {
				case client.send <- msg:
				default:
					return
				}
			}
		}
	}()

	// Reader goroutine (keep connection alive)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (s *Server) collectRealtimeMetrics() map[string]interface{} {
	result := map[string]interface{}{
		"timestamp": time.Now().UTC(),
	}

	queries := map[string]string{
		"qps":             `sum(rate(kresd_query_total[1m]))`,
		"avg_latency_ms":  `histogram_quantile(0.5, rate(kresd_answer_duration_seconds_bucket[1m])) * 1000`,
		"cache_hit_ratio": `rate(kresd_cache_hit_total[1m]) / (rate(kresd_cache_hit_total[1m]) + rate(kresd_cache_miss_total[1m]))`,
		"cpu_usage":       `100 - (avg(rate(node_cpu_seconds_total{mode="idle"}[1m])) * 100)`,
		"memory_used_pct": `(1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes) * 100`,
	}

	for name, query := range queries {
		data, err := s.promInstantQuery(query)
		if err == nil {
			result[name] = json.RawMessage(data)
		}
	}

	return result
}
