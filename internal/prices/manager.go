package prices

import (
	"github.com/jasonmichels/Market-Sentry/internal/sse"
	"log"
	"sync"

	"github.com/jasonmichels/Market-Sentry/internal/storage"
)

// UpdatePriceStore updates the in-memory store with fresh data:
//  1. Gather unique symbols from active alerts
//  2. Fetch them from relevant APIs
//  3. Store them
//  4. Trigger any alerts that meet conditions
func UpdatePriceStore(store *storage.MemoryStore, hub *sse.SSEHub) {
	log.Println("[Price Fetch] Starting update...")

	// 1) Gather needed symbols from the store
	cryptoSymbols, metalSymbols, stockSymbols := gatherSymbols(store)

	// If everything is empty, log and bail out early
	if len(cryptoSymbols) == 0 && len(metalSymbols) == 0 && len(stockSymbols) == 0 {
		log.Println("[Price Fetch] No symbols to fetch. Skipping API calls.")
		// We can still do an alert check, but there's nothing new
		return
	}

	// 2) Fetch them from relevant APIs concurrently
	var wg sync.WaitGroup
	var cryptoErr, metalErr, stockErr error
	var cryptoData, metalData, stockData map[string]float64

	wg.Add(3)

	// (A) Crypto
	go func() {
		defer wg.Done()
		if len(cryptoSymbols) == 0 {
			log.Println("[Crypto Fetch] No crypto symbols to fetch.")
			return
		}
		cryptoData, cryptoErr = FetchCrypto(cryptoSymbols)
		if cryptoErr != nil {
			log.Printf("[Error] Failed to fetch crypto data: %v\n", cryptoErr)
		} else {
			log.Printf("[Crypto Fetch] Fetched data: %v\n", cryptoData)
		}
	}()

	// (B) Metals
	go func() {
		defer wg.Done()
		if len(metalSymbols) == 0 {
			log.Println("[Metals Fetch] No metal symbols to fetch.")
			return
		}
		metalData, metalErr = FetchMetals(metalSymbols)
		if metalErr != nil {
			log.Printf("[Error] Failed to fetch metals data: %v\n", metalErr)
		} else {
			log.Printf("[Metals Fetch] Fetched data: %v\n", metalData)
		}
	}()

	// (C) Stocks
	go func() {
		defer wg.Done()
		if len(stockSymbols) == 0 {
			log.Println("[Stocks Fetch] No stock symbols to fetch.")
			return
		}
		stockData, stockErr = FetchStocks(stockSymbols)
		if stockErr != nil {
			log.Printf("[Error] Failed to fetch stocks data: %v\n", stockErr)
		} else {
			log.Printf("[Stocks Fetch] Fetched data: %v\n", stockData)
		}
	}()

	wg.Wait()

	// 3) Lock store and update
	store.Mu.Lock()
	if cryptoErr == nil && cryptoData != nil {
		store.Crypto = cryptoData
	}
	if metalErr == nil && metalData != nil {
		store.Metals = metalData
	}
	if stockErr == nil && stockData != nil {
		store.Stocks = stockData
	}
	store.Mu.Unlock()

	log.Println("[Price Fetch] Update complete. Checking alerts...")

	// 4) Trigger any alerts if necessary
	TriggerAlerts(store, hub)
}

// gatherSymbols scans all users’ ActiveAlerts to find needed symbols for each asset type
func gatherSymbols(store *storage.MemoryStore) (cryptoSymbols, metalSymbols, stockSymbols []string) {
	cryptoSet := make(map[string]bool)
	metalSet := make(map[string]bool)
	stockSet := make(map[string]bool)

	store.Mu.RLock()
	defer store.Mu.RUnlock()

	for _, user := range store.Users {
		for _, alert := range user.ActiveAlerts {
			switch alert.AssetType {
			case "crypto":
				cryptoSet[alert.Symbol] = true
			case "metal":
				metalSet[alert.Symbol] = true
			case "stock":
				stockSet[alert.Symbol] = true
			}
		}
	}

	// Convert sets to slices
	for sym := range cryptoSet {
		cryptoSymbols = append(cryptoSymbols, sym)
	}
	for sym := range metalSet {
		metalSymbols = append(metalSymbols, sym)
	}
	for sym := range stockSet {
		stockSymbols = append(stockSymbols, sym)
	}

	return
}
