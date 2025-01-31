package main

import (
	"github.com/jasonmichels/Market-Sentry/internal/sse"
	"log"
	"net/http"
	"time"

	// Standard Go Modules
	"html/template"

	// Internal packages
	"github.com/jasonmichels/Market-Sentry/internal/alerts"
	"github.com/jasonmichels/Market-Sentry/internal/auth"
	"github.com/jasonmichels/Market-Sentry/internal/metrics"
	"github.com/jasonmichels/Market-Sentry/internal/prices"
	"github.com/jasonmichels/Market-Sentry/internal/storage"

	// Prometheus
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Initialize global in-memory store
	store := storage.NewMemoryStore()

	// Create SSE hub
	hub := sse.NewSSEHub()

	// Start background goroutine to fetch prices every 10 min
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			// We can fetch crypto, metals, stocks in parallel or sequentially
			prices.UpdatePriceStore(store, hub)
			<-ticker.C
		}
	}()
	// SSE endpoint
	http.Handle("/alerts/stream", hub)

	// Set up HTTP handlers
	// You can also use a router like gorilla/mux if you prefer, but let's keep it minimal.

	// Serve Prometheus metrics at /metrics
	http.Handle("/metrics", promhttp.Handler())

	// Serve static files if needed
	// http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))
	// In main.go, before http.ListenAndServe
	http.Handle("/data/", http.StripPrefix("/data/", http.FileServer(http.Dir("./web/data"))))

	// Public routes
	http.HandleFunc("/", makeHandler(store, handleHome))
	http.HandleFunc("/login", makeHandler(store, handleLogin))
	http.HandleFunc("/login/verify", makeHandler(store, handleVerify))

	// Protected routes (require JWT)
	http.Handle("/alerts", auth.JWTMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAlerts(store, w, r)
	})))

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// makeHandler is just a helper to track requests in Prometheus
func makeHandler(store *storage.MemoryStore, fn func(*storage.MemoryStore, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics.TotalRequests.Inc()
		fn(store, w, r)
	}
}

// handleHome - display a simple home page with a link to login
func handleHome(store *storage.MemoryStore, w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("web/templates/login.html"))
	_ = tmpl.Execute(w, nil)
}

// handleLogin - user enters phone number, we generate a one-time code
func handleLogin(store *storage.MemoryStore, w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		phone := r.FormValue("phone")

		// Store or create user in memory
		user := store.GetOrCreateUser(phone)

		// Generate a one-time code for demonstration (in real life: SMS!)
		user.OneTimeCode = "123456" // super insecure placeholder
		user.OneTimeCodeExpires = time.Now().Add(5 * time.Minute)

		log.Printf("One-time code for user %s is: %s\n", phone, user.OneTimeCode)

		// For demonstration, we just tell them to go to /login/verify
		http.Redirect(w, r, "/login/verify?phone="+phone, http.StatusSeeOther)
		return
	}

	// Render basic form to collect phone number
	tmpl := template.Must(template.ParseFiles("web/templates/login.html"))
	_ = tmpl.Execute(w, nil)
}

// handleVerify - user enters the one-time code
func handleVerify(store *storage.MemoryStore, w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phone")

	if r.Method == http.MethodPost {
		// They submitted the code
		code := r.FormValue("code")
		user := store.Users[phone]
		if user == nil {
			http.Error(w, "User not found", http.StatusBadRequest)
			return
		}
		if user.OneTimeCode == code && time.Now().Before(user.OneTimeCodeExpires) {
			// Success, generate JWT
			tokenStr, err := auth.GenerateJWT(phone)
			if err != nil {
				log.Println("Error generating JWT:", err)
				http.Error(w, "Failed to generate token", http.StatusInternalServerError)
				return
			}

			// Set JWT as cookie
			http.SetCookie(w, &http.Cookie{
				Name:     "token",
				Value:    tokenStr,
				Path:     "/",
				Expires:  time.Now().Add(8 * time.Hour),
				HttpOnly: true,  // safer from XSS
				Secure:   false, // set true in production (HTTPS)
			})

			// Reset the code
			user.OneTimeCode = ""
			user.OneTimeCodeExpires = time.Time{}

			// Redirect to alerts
			http.Redirect(w, r, "/alerts", http.StatusSeeOther)
			return
		}
		http.Error(w, "Invalid code or expired", http.StatusUnauthorized)
		return
	}

	// If GET, they want to see the verification form.
	// Pass the phone into the template data so {{.Phone}} is populated.
	data := struct {
		Phone string
	}{
		Phone: phone,
	}

	tmpl := template.Must(template.ParseFiles("web/templates/verify-phone.html"))
	_ = tmpl.Execute(w, data)
}

// handleAlerts - for authenticated users, show the current alerts and a form to create new ones
func handleAlerts(store *storage.MemoryStore, w http.ResponseWriter, r *http.Request) {
	phone := auth.GetUserPhone(r.Context()) // from JWT middleware
	user := store.Users[phone]
	if user == nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	if r.Method == http.MethodPost {
		// Create new alert
		assetType := r.FormValue("assetType") // "crypto", "metal", or "stock"
		symbol := r.FormValue("symbol")
		threshold := r.FormValue("threshold")
		direction := r.FormValue("direction") // "above" or "below"

		alerts.CreateAlert(store, phone, assetType, symbol, threshold, direction)
		http.Redirect(w, r, "/alerts", http.StatusSeeOther)
		return
	}

	// Show list of active alerts + triggered alerts
	tmpl := template.Must(template.ParseFiles("web/templates/alerts.html"))
	_ = tmpl.Execute(w, user)
}
