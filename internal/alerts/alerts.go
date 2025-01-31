package alerts

import (
	"github.com/google/uuid"
	"github.com/jasonmichels/Market-Sentry/internal/storage"
	"log"
	"strconv"
)

func CreateAlert(store *storage.MemoryStore, phone, assetType, symbol, thresholdStr, direction string) {
	store.Mu.Lock()
	defer store.Mu.Unlock()

	user := store.Users[phone]
	if user == nil {
		log.Printf("User %s not found\n", phone)
		return
	}

	threshold, err := strconv.ParseFloat(thresholdStr, 64)
	if err != nil {
		log.Println("Error parsing threshold:", err)
		return
	}

	above := direction == "above"

	alert := storage.Alert{
		ID:        generateAlertID(),
		AssetType: assetType,
		Symbol:    symbol,
		Threshold: threshold,
		Above:     above,
	}

	user.ActiveAlerts = append(user.ActiveAlerts, alert)

	user.CountActiveAlerts++
}

// generateAlertID
func generateAlertID() string {
	return uuid.New().String()
}
