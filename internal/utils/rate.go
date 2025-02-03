package utils

import (
	"golang.org/x/time/rate"
	"sync"
)

var (
	loginLimiters  = make(map[string]*rate.Limiter)
	loginLimiterMu sync.Mutex
)

// GetLoginLimiter returns the rate limiter for a given phone, creating one if needed.
// It allows 20 requests per day (20/86400 tokens per second) with a burst capacity of 20.
func GetLoginLimiter(phone string) *rate.Limiter {
	loginLimiterMu.Lock()
	defer loginLimiterMu.Unlock()
	limiter, exists := loginLimiters[phone]
	if !exists {
		// 20 tokens per day equals 20/86400 tokens per second.
		limiter = rate.NewLimiter(rate.Limit(20.0/86400.0), 20)
		loginLimiters[phone] = limiter
	}
	return limiter
}
