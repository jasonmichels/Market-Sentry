package routes

import (
	"fmt"
	"github.com/jasonmichels/Market-Sentry/internal/alerts"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jasonmichels/Market-Sentry/internal/auth"
	"github.com/jasonmichels/Market-Sentry/internal/sse"
	"github.com/jasonmichels/Market-Sentry/internal/storage"
)

// tmplFuncs is shared by alerts pages.
var tmplFuncs = template.FuncMap{
	"formatTime": func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Format("Jan 2 2006 3:04 PM")
	},
}

// RegisterAlertsRoutes registers alerts-related routes.
func RegisterAlertsRoutes(mux *http.ServeMux, store *storage.MemoryStore, hub *sse.SSEHub) {
	mux.Handle("/alerts", auth.JWTMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAlerts(store, w, r)
	})))
	mux.Handle("/alerts/partial", auth.JWTMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleAlertsPartial(store, w, r)
	})))
	mux.Handle("/alerts/stream", auth.JWTMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeHTTP(w, r)
	})))
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
