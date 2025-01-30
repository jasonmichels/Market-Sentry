package prices

import "log"

// FetchStocks calls a stocks API with a list of symbols
func FetchStocks(symbols []string) (map[string]float64, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	log.Printf("[Stocks Fetch] Requested symbols: %v\n", symbols)

	// TODO: call a real stocks API
	data := make(map[string]float64)
	for _, sym := range symbols {
		switch sym {
		case "AAPL":
			data[sym] = 150.25
		case "TSLA":
			data[sym] = 730.10
		default:
			data[sym] = 100.00 // placeholder
		}
	}
	return data, nil
}
