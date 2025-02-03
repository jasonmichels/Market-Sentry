package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	// Internal packages
	"github.com/jasonmichels/Market-Sentry/internal/prices"
	"github.com/jasonmichels/Market-Sentry/internal/sse"
	"github.com/jasonmichels/Market-Sentry/internal/storage"

	// Our new route packages
	"github.com/jasonmichels/Market-Sentry/internal/routes"

	// Prometheus
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// global adminPhones map
var adminPhones = make(map[string]bool)

func main() {
	// Read ADMIN_PHONES from env (comma-separated)
	adminEnv := os.Getenv("ADMIN_PHONES")
	if adminEnv != "" {
		for _, p := range strings.Split(adminEnv, ",") {
			adminPhones[strings.TrimSpace(p)] = true
		}
	}

	// Load supported coins from coins.json
	supportedCoins, err := loadSupportedCoins("web/data/coins.json")
	if err != nil {
		log.Fatalf("Failed to load coins.json: %v", err)
	}

	// Initialize global in-memory store (assume NewMemoryStore accepts supportedCoins)
	store := storage.NewMemoryStore(supportedCoins)

	// Create SSE hub
	hub := sse.NewSSEHub()

	// Start background goroutine for price updates every 10 minutes
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			prices.UpdatePriceStore(store, hub)
			<-ticker.C
		}
	}()

	// Create a new ServeMux
	mux := http.NewServeMux()

	// Register routes from our route files
	routes.RegisterAuthRoutes(mux, store)
	routes.RegisterAlertsRoutes(mux, store, hub)
	routes.RegisterAdminRoutes(mux, store, adminPhones)

	// Additional routes: static files and Prometheus metrics
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/data/", http.StripPrefix("/data/", http.FileServer(http.Dir("./web/data"))))

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

// loadSupportedCoins reads coins.json and returns a map of coin IDs to true.
func loadSupportedCoins(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	type Coin struct {
		ID     string `json:"id"`
		Symbol string `json:"symbol"`
		Name   string `json:"name"`
	}
	var coins []Coin
	if err := json.Unmarshal(data, &coins); err != nil {
		return nil, err
	}
	supported := make(map[string]bool, len(coins))
	for _, c := range coins {
		supported[c.ID] = true
	}
	return supported, nil
}
