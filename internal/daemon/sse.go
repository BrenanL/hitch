package daemon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// SSEHub manages fan-out broadcasting of events to connected SSE clients.
type SSEHub struct {
	mu      sync.RWMutex
	clients map[string]map[chan DaemonEvent]struct{} // session_id -> set of channels
}

// DaemonEvent is a structured event broadcast via SSE and the events endpoint.
type DaemonEvent struct {
	Timestamp   string `json:"ts"`
	Source      string `json:"source"`     // "proxy", "hooks", "jsonl"
	EventType   string `json:"type"`       // "api_request", "hook_event", "subagent_start", etc.
	SessionID   string `json:"session_id"`
	Description string `json:"description"`
}

// NewSSEHub creates an empty hub.
func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[string]map[chan DaemonEvent]struct{}),
	}
}

// Subscribe registers a client channel for events on a specific session.
// Returns a channel that receives events and an unsubscribe function.
func (h *SSEHub) Subscribe(sessionID string) (<-chan DaemonEvent, func()) {
	ch := make(chan DaemonEvent, 1000) // buffered to avoid blocking

	h.mu.Lock()
	if h.clients[sessionID] == nil {
		h.clients[sessionID] = make(map[chan DaemonEvent]struct{})
	}
	h.clients[sessionID][ch] = struct{}{}
	h.mu.Unlock()

	unsub := func() {
		h.mu.Lock()
		delete(h.clients[sessionID], ch)
		if len(h.clients[sessionID]) == 0 {
			delete(h.clients, sessionID)
		}
		h.mu.Unlock()
		// Drain any remaining events
		for {
			select {
			case <-ch:
			default:
				return
			}
		}
	}

	return ch, unsub
}

// Broadcast sends an event to all clients subscribed to the given session.
func (h *SSEHub) Broadcast(sessionID string, evt DaemonEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := h.clients[sessionID]
	for ch := range clients {
		select {
		case ch <- evt:
		default:
			// Channel full — slow consumer, drop event
		}
	}
}

// ClientCount returns the number of connected clients for a session.
func (h *SSEHub) ClientCount(sessionID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[sessionID])
}

// serveSSE handles an SSE stream connection for a session.
func (d *Daemon) serveSSE(w http.ResponseWriter, r *http.Request, sessionID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch, unsub := d.SSEHub.Subscribe(sessionID)
	defer unsub()

	// Heartbeat ticker
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt := <-ch:
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}
