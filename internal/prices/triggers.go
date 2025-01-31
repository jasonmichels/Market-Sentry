package prices

import (
	"fmt"
	"github.com/jasonmichels/Market-Sentry/internal/sse"
	"log"
	"time"

	"github.com/jasonmichels/Market-Sentry/internal/storage"
)

// TriggerAlerts checks each user's ActiveAlerts against the current prices
// and moves triggered alerts + builds notifications.
func TriggerAlerts(store *storage.MemoryStore, hub *sse.SSEHub) {
	// We'll need the prices to check the conditions
	store.Mu.RLock()
	cryptoPrices := store.Crypto
	metalPrices := store.Metals
	stockPrices := store.Stocks
	usersMap := store.Users
	store.Mu.RUnlock()

	// triggeredMap[phone] = slice of triggered alert IDs
	triggeredMap := make(map[string][]string)
	// notificationsMap[phone] = slice of notifications
	notificationsMap := make(map[string][]storage.Notification)

	for phone, user := range usersMap {
		for _, alert := range user.ActiveAlerts {
			price := getPriceForAlert(alert, cryptoPrices, metalPrices, stockPrices)
			if price == 0 {
				// If we don't have a price, skip
				continue
			}
			// Check condition
			triggered := false
			if alert.Above && price > alert.Threshold {
				triggered = true
			} else if !alert.Above && price < alert.Threshold {
				triggered = true
			}

			if triggered {
				log.Printf("[Alert Trigger] Phone=%s Symbol=%s Price=%.2f Threshold=%.2f Above=%t",
					phone, alert.Symbol, price, alert.Threshold, alert.Above)

				triggeredMap[phone] = append(triggeredMap[phone], alert.ID)

				// Build a notification
				msg := ""
				if alert.Above {
					msg = fmt.Sprintf("%s went above %.2f", alert.Symbol, alert.Threshold)
				} else {
					msg = fmt.Sprintf("%s went below %.2f", alert.Symbol, alert.Threshold)
				}
				notificationsMap[phone] = append(notificationsMap[phone], storage.Notification{
					AlertID:   alert.ID,
					Timestamp: time.Now(),
					Message:   msg,
				})
			}
		}
	}

	// Now apply the triggered changes under a write lock
	store.Mu.Lock()
	defer store.Mu.Unlock()

	for phone, alertIDs := range triggeredMap {
		user := store.Users[phone]
		if user == nil {
			continue
		}

		// Move triggered alerts from ActiveAlerts to TriggeredAlerts
		var remaining []storage.Alert
		for _, a := range user.ActiveAlerts {
			triggered := false
			for _, trigID := range alertIDs {
				if a.ID == trigID {
					user.TriggeredAlerts = append(user.TriggeredAlerts, a)
					triggered = true
					break
				}
			}
			if !triggered {
				remaining = append(remaining, a)
			}
		}
		user.ActiveAlerts = remaining

		// Add new notifications
		notes := notificationsMap[phone]
		user.Notifications = append(user.Notifications, notes...)
	}

	// >>> BROADCAST after updates done
	hub.Broadcast(`{"type":"alertsUpdated","message":"Alerts updated"}`)

}

// getPriceForAlert returns the relevant price for the alert from the store
func getPriceForAlert(alert storage.Alert, crypto map[string]float64, metals map[string]float64, stocks map[string]float64) float64 {
	switch alert.AssetType {
	case "crypto":
		return crypto[alert.Symbol]
	case "metal":
		return metals[alert.Symbol]
	case "stock":
		return stocks[alert.Symbol]
	}
	return 0
}
