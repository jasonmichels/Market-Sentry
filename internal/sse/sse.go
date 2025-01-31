// sse.go (new file in internal/sse or similar)
package sse

import (
	"fmt"
	"net/http"
	"sync"
)

// SSEClient holds a channel we can use to push data to the client.
type SSEClient struct {
	chanStream chan string
	done       chan struct{}
}

// SSEHub tracks connected clients and provides a way to broadcast updates.
type SSEHub struct {
	mu      sync.RWMutex
	clients map[*SSEClient]bool
}

// NewSSEHub initializes an SSEHub.
func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[*SSEClient]bool),
	}
}

// AddClient registers a new SSE client
func (hub *SSEHub) AddClient() *SSEClient {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	c := &SSEClient{
		chanStream: make(chan string, 10), // buffered channel
		done:       make(chan struct{}),
	}
	hub.clients[c] = true
	return c
}

// RemoveClient unregisters a client
func (hub *SSEHub) RemoveClient(c *SSEClient) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	delete(hub.clients, c)
	close(c.done)
}

// Broadcast sends a string message to all connected clients
func (hub *SSEHub) Broadcast(msg string) {
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	for c := range hub.clients {
		// Non-blocking send to avoid slow or stuck clients blocking everyone
		select {
		case c.chanStream <- msg:
		default:
			// If their channel is full, skip
			// (could also remove them if you want)
		}
	}
}

// ServeHTTP implements an SSE endpoint
func (hub *SSEHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Make sure to set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Add the new client
	client := hub.AddClient()
	defer hub.RemoveClient(client)

	// Close the connection when client disconnects
	notify := w.(http.CloseNotifier).CloseNotify()

	// Continuously flush events to the client
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case <-notify:
			// Client closed connection
			return

		case <-client.done:
			// Server forcibly removed this client
			return

		case msg := <-client.chanStream:
			// SSE requires the "data:" prefix. Then a blank line
			_, err := fmt.Fprintf(w, "data: %s\n\n", msg)
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
