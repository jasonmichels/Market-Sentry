package prices

import (
	"log"
)

// FetchMetals calls a metals API with a list of symbols
func FetchMetals(symbols []string) (map[string]float64, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	log.Printf("[Metals Fetch] Requested symbols: %v\n", symbols)

	// TODO: call actual free metals API, pass your symbols
	// For now, return dummy data matching each symbol
	data := make(map[string]float64)
	for _, sym := range symbols {
		switch sym {
		case "gold":
			data[sym] = 1940.15
		case "silver":
			data[sym] = 25.30
		default:
			// Just a placeholder
			data[sym] = 99.99
		}
	}
	return data, nil
}
