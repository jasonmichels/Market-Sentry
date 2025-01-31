package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	// Standard Go Modules
	"html/template"

	// Internal packages
	"github.com/jasonmichels/Market-Sentry/internal/alerts"
	"github.com/jasonmichels/Market-Sentry/internal/auth"
	"github.com/jasonmichels/Market-Sentry/internal/metrics"
	"github.com/jasonmichels/Market-Sentry/internal/prices"
	"github.com/jasonmichels/Market-Sentry/internal/sse"
	"github.com/jasonmichels/Market-Sentry/internal/storage"

	// Prometheus
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Load all supported coin IDs from coins.json
	supportedCoins, err := loadSupportedCoins("web/data/coins.json")
	if err != nil {
		log.Fatalf("Failed to load coins.json: %v", err)
	}

	// Initialize global in-memory store
	store := storage.NewMemoryStore(supportedCoins)

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

	// Expose SSE endpoint (protected by JWT)
	// So only logged-in users can connect to SSE
	http.Handle("/alerts/stream", auth.JWTMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeHTTP(w, r)
	})))

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
	// Protected route: returns just the HTML partial for alerts/notifications
	http.Handle("/alerts/partial", auth.JWTMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAlertsPartial(store, w, r)
	})))

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// loadSupportedCoins reads the coins.json file (array of {id, symbol, name})
// and returns a map of coin "id" -> true for fast lookup.
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
	// The user is using "symbol" as the hidden field value?
	// Actually, we said the hidden field is "coin.id" (like "bitcoin").
	// Confirm which field you're actually validating: "symbol" == coin.id?
	// If so, let's store coin.ID. If you truly match on ID, do:
	for _, c := range coins {
		supported[c.ID] = true
	}
	return supported, nil
}

var tmplFuncs = template.FuncMap{
	"formatTime": func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Format("Jan 2 2006 3:04 PM")
	},
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
			http.Redirect(w, r, "/", http.StatusSeeOther)
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

func handleAlerts(store *storage.MemoryStore, w http.ResponseWriter, r *http.Request) {
	phone := auth.GetUserPhone(r.Context()) // from JWT middleware
	user := store.Users[phone]
	if user == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// If POST, we are creating a new alert
	if r.Method == http.MethodPost {
		assetType := r.FormValue("assetType")    // "crypto", "metal", "stock"
		symbol := r.FormValue("symbol")          // e.g. "bitcoin"
		thresholdStr := r.FormValue("threshold") // e.g. "20000"
		direction := r.FormValue("direction")    // "above" or "below"

		var validationErrors []string

		// 1) Validate assetType
		validAssetTypes := map[string]bool{
			"crypto": true,
			"metal":  true,
			"stock":  true,
		}
		if !validAssetTypes[assetType] {
			validationErrors = append(validationErrors, "Invalid asset type.")
		}

		if assetType == "crypto" {
			store.Mu.RLock()
			_, found := store.SupportedCoins[symbol]
			store.Mu.RUnlock()

			if !found {
				validationErrors = append(validationErrors, fmt.Sprintf("Invalid crypto coin: %s", symbol))
			}
		}

		// 3) Validate threshold is a numeric > 0
		thresholdVal, err := strconv.ParseFloat(thresholdStr, 64)
		if err != nil || thresholdVal <= 0 {
			validationErrors = append(validationErrors, "Threshold must be a valid number greater than 0.")
		}

		// 4) Validate direction
		if direction != "above" && direction != "below" {
			validationErrors = append(validationErrors, "Invalid direction, must be 'above' or 'below'.")
		}

		// If any validation errors, re-render alertsPage with error messages
		if len(validationErrors) > 0 {
			// Build the same template we normally show, but inject an Errors field
			// plus the form fields so the user doesn't lose what they typed.
			data := struct {
				User          *storage.User
				Crypto        map[string]float64
				Errors        []string
				FormAssetType string
				FormSymbol    string
				FormThreshold string
				FormDirection string
			}{
				User:          user,
				Crypto:        store.Crypto,
				Errors:        validationErrors,
				FormAssetType: assetType,
				FormSymbol:    symbol,
				FormThreshold: thresholdStr,
				FormDirection: direction,
			}

			// parse + render
			tmpl := template.Must(template.New("alerts.html").
				Funcs(tmplFuncs).
				ParseFiles(
					"web/templates/alerts.html",
					"web/templates/alerts_partial.html",
				))
			if err := tmpl.ExecuteTemplate(w, "alertsPage", data); err != nil {
				log.Printf("Error executing template with validation errors: %v", err)
				http.Error(w, "Server error", http.StatusInternalServerError)
			}
			return
		}

		// If we get here, everything is valid -> create alert
		alerts.CreateAlert(store, phone, assetType, symbol, thresholdStr, direction)
		http.Redirect(w, r, "/alerts", http.StatusSeeOther)
		return
	}

	// If GET, show the alerts page (no errors)
	data := struct {
		User          *storage.User
		Crypto        map[string]float64
		Errors        []string
		FormAssetType string
		FormSymbol    string
		FormThreshold string
		FormDirection string
	}{
		User:          user,
		Crypto:        store.Crypto,
		Errors:        nil,
		FormAssetType: "crypto", // default
		FormSymbol:    "",
		FormThreshold: "",
		FormDirection: "above",
	}

	tmpl := template.Must(template.New("alerts.html").
		Funcs(tmplFuncs).
		ParseFiles(
			"web/templates/alerts.html",
			"web/templates/alerts_partial.html",
		))
	if err := tmpl.ExecuteTemplate(w, "alertsPage", data); err != nil {
		log.Printf("Error executing alertsPage template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func handleAlertsPartial(store *storage.MemoryStore, w http.ResponseWriter, r *http.Request) {
	phone := auth.GetUserPhone(r.Context())
	if phone == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user := store.Users[phone]
	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// If user slices might be nil, ensure they're empty so we don't get null in templates
	if user.ActiveAlerts == nil {
		user.ActiveAlerts = []storage.Alert{}
	}
	if user.TriggeredAlerts == nil {
		user.TriggeredAlerts = []storage.Alert{}
	}
	if user.Notifications == nil {
		user.Notifications = []storage.Notification{}
	}

	tmpl := template.Must(template.New("alerts.html").
		Funcs(tmplFuncs).
		ParseFiles(
			"web/templates/alerts.html",
			"web/templates/alerts_partial.html",
		),
	)

	// Build the same data struct you used in handleAlerts
	data := struct {
		User   *storage.User
		Crypto map[string]float64
	}{
		User:   user,
		Crypto: store.Crypto, // store.Crypto is the map of prices
	}

	// Now render only the "alertsPartial" template block
	// (since we just want the snippet)
	if err := tmpl.ExecuteTemplate(w, "alertsPartial", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
