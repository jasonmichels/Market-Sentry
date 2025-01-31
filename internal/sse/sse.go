package sse

import (
	"fmt"
	"github.com/jasonmichels/Market-Sentry/internal/auth"
	"net/http"
	"sync"
)

// SSEClient holds a channel for sending data and a signal when done.
type SSEClient struct {
	chanStream chan string
	done       chan struct{}
}

// SSEHub tracks all active SSE connections, organized by phone.
//
//	phone -> map of SSEClient -> bool
type SSEHub struct {
	mu      sync.RWMutex
	clients map[string]map[*SSEClient]bool
}

// NewSSEHub initializes an SSEHub.
func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[string]map[*SSEClient]bool),
	}
}

// AddClient registers a new client for a given phone.
func (hub *SSEHub) AddClient(phone string) *SSEClient {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	c := &SSEClient{
		chanStream: make(chan string, 10), // some buffering
		done:       make(chan struct{}),
	}

	if _, ok := hub.clients[phone]; !ok {
		hub.clients[phone] = make(map[*SSEClient]bool)
	}
	hub.clients[phone][c] = true
	return c
}

// RemoveClient unregisters a client for that phone.
func (hub *SSEHub) RemoveClient(phone string, c *SSEClient) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if clientSet, ok := hub.clients[phone]; ok {
		delete(clientSet, c)
		if len(clientSet) == 0 {
			// no more clients for this phone
			delete(hub.clients, phone)
		}
	}
	close(c.done)
}

// BroadcastToUser sends a message only to the SSE clients for a given phone.
func (hub *SSEHub) BroadcastToUser(phone, msg string) {
	hub.mu.RLock()
	defer hub.mu.RUnlock()

	clientSet, ok := hub.clients[phone]
	if !ok {
		return // no active connections for that user
	}

	for c := range clientSet {
		select {
		case c.chanStream <- msg:
			// message enqueued
		default:
			// channel is full or blocked; skip or remove
		}
	}
}

// ServeHTTP is the endpoint that an authenticated user hits to establish SSE.
func (hub *SSEHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Mark the headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Grab phone from context (Populated by JWTMiddleware)
	ctx := r.Context()
	phone := auth.GetUserPhone(r.Context()) // uses the same typed key as middleware
	if phone == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Create a new client for this phone
	client := hub.AddClient(phone)
	defer hub.RemoveClient(phone, client)

	// Get the Flusher interface
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// Listen for:
	//  1) request cancellation (client closed connection)
	//  2) new messages for this client
	for {
		select {
		case <-ctx.Done():
			// The client disconnected or request was canceled
			return

		case msg := <-client.chanStream:
			// SSE format: "data: <message>\n\n"
			_, err := fmt.Fprintf(w, "data: %s\n\n", msg)
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
