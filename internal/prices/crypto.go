package prices

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// coingeckoResponse is the example response shape for multiple coins
type coingeckoResponse map[string]struct {
	USD float64 `json:"usd"`
}

// FetchCrypto calls CoinGecko with a list of symbols (e.g. "bitcoin", "ethereum")
func FetchCrypto(symbols []string) (map[string]float64, error) {
	if len(symbols) == 0 {
		return nil, nil
	}

	client := &http.Client{Timeout: 5 * time.Second}

	// Example: "bitcoin,ethereum,dogecoin"
	joined := strings.Join(symbols, ",")
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd", joined)

	log.Printf("[Crypto Fetch] Sending request to CoinGecko: %s\n", url)

	resp, err := client.Get(url)
	if err != nil {
		log.Printf("[Error] CoinGecko request failed: %v\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("[Crypto Fetch] Received response with status: %s\n", resp.Status)

	// Decode response
	var cgResp coingeckoResponse
	if err := json.NewDecoder(resp.Body).Decode(&cgResp); err != nil {
		log.Printf("[Error] Failed to decode CoinGecko response: %v\n", err)
		return nil, err
	}

	// Build output
	out := make(map[string]float64)
	for coin, data := range cgResp {
		out[coin] = data.USD
	}

	log.Printf("[Crypto Fetch] Processed data: %v\n", out)
	return out, nil
}
