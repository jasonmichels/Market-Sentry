package routes

import (
	"crypto/rand"
	"fmt"
	"github.com/jasonmichels/Market-Sentry/internal/utils"
	"html/template"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jasonmichels/Market-Sentry/internal/auth"
	"github.com/jasonmichels/Market-Sentry/internal/storage"
	"github.com/jasonmichels/Market-Sentry/internal/twilio" // your new twilio module
)

// generateOneTimeCode generates a random 6-character alphanumeric code.
func generateOneTimeCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, 6)
	for i := range code {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			code[i] = charset[0]
		} else {
			code[i] = charset[num.Int64()]
		}
	}
	return string(code)
}

// RegisterAuthRoutes registers routes for public authentication.
func RegisterAuthRoutes(mux *http.ServeMux, store *storage.MemoryStore) {
	// Home/Landing Page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("web/templates/index.html"))
		_ = tmpl.Execute(w, nil)
	})

	// /login route: Validate phone, generate one-time code, and send via Twilio.
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Expect both "phone" and "country" in the form.
			phoneInput := r.FormValue("phone")
			countryCode := r.FormValue("country")
			log.Printf("Received login request for phone: %s, country: %s", phoneInput, countryCode)
			if countryCode == "" {
				countryCode = "+1" // Default to United States country code
			}

			// Validate and normalize the phone number with the provided country code.
			phone, valid := utils.ValidatePhoneWithCountry(phoneInput, countryCode)
			if !valid {
				http.Error(w, "Invalid phone number format", http.StatusBadRequest)
				return
			}

			// --- RATE LIMITING USING go-rate ---
			environment := os.Getenv("ENVIRONMENT")
			if environment != "local" {
				limiter := utils.GetLoginLimiter(phone)
				if !limiter.Allow() {
					http.Error(w, "Too many login attempts. Please try again later.", http.StatusTooManyRequests)
					return
				}
			}
			// --- END RATE LIMITING ---

			// Get or create the user.
			user := store.GetOrCreateUser(phone)
			// Generate a 6-character one-time code.
			code := generateOneTimeCode()
			user.OneTimeCode = code
			user.OneTimeCodeExpires = time.Now().Add(5 * time.Minute)

			log.Printf("Generated OneTimeCode for user %s: %s", phone, code)

			// Build the SMS message.
			message := fmt.Sprintf("Your Market Sentry verification code is: %s", code)
			// Send the SMS via the Twilio module.
			if err := twilio.SendSMS(phone, message); err != nil {
				log.Printf("Error sending SMS to %s: %v", phone, err)
				http.Error(w, "Failed to send verification code", http.StatusInternalServerError)
				return
			}

			redirectURL := "/login/verify?phone=" + url.QueryEscape(phone)
			http.Redirect(w, r, redirectURL, http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// /login/verify route: Let the user enter the code.
	mux.HandleFunc("/login/verify", func(w http.ResponseWriter, r *http.Request) {
		phone := r.URL.Query().Get("phone")
		if r.Method == http.MethodPost {
			code := strings.ToUpper(r.FormValue("code"))
			user := store.Users[phone]
			log.Printf("Verifying code for user %s: %s", phone, code)
			if user == nil {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			if user.OneTimeCode == code && time.Now().Before(user.OneTimeCodeExpires) {
				tokenStr, err := auth.GenerateJWT(phone)
				if err != nil {
					log.Println("Error generating JWT:", err)
					http.Error(w, "Failed to generate token", http.StatusInternalServerError)
					return
				}
				http.SetCookie(w, &http.Cookie{
					Name:     "marketsentry",
					Value:    tokenStr,
					Path:     "/",
					Expires:  time.Now().Add(8 * time.Hour),
					HttpOnly: true,
					Secure:   false,
				})
				// Clear the one-time code
				user.OneTimeCode = ""
				user.OneTimeCodeExpires = time.Time{}
				http.Redirect(w, r, "/alerts", http.StatusSeeOther)
				return
			}
			http.Error(w, "Invalid code or expired", http.StatusUnauthorized)
			return
		}
		tmpl := template.Must(template.ParseFiles("web/templates/verify-phone.html"))
		data := struct{ Phone string }{Phone: phone}
		_ = tmpl.Execute(w, data)
	})
}
