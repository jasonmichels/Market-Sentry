package storage

import (
	"sync"
	"time"
)

type User struct {
	PhoneNumber        string
	OneTimeCode        string
	OneTimeCodeExpires time.Time

	// Active and triggered alerts
	ActiveAlerts    []Alert
	TriggeredAlerts []Alert

	Notifications []Notification
}

type Alert struct {
	ID        string
	AssetType string // "crypto", "metal", "stock"
	Symbol    string
	Threshold float64
	Above     bool // true = alert if price > threshold, false = alert if price < threshold
}

type Notification struct {
	AlertID   string
	Timestamp time.Time
	Message   string
}

type MemoryStore struct {
	Mu    sync.RWMutex
	Users map[string]*User

	// Price data (updated every 10 min)
	Crypto map[string]float64
	Metals map[string]float64
	Stocks map[string]float64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		Users:  make(map[string]*User),
		Crypto: make(map[string]float64),
		Metals: make(map[string]float64),
		Stocks: make(map[string]float64),
	}
}

func (ms *MemoryStore) GetOrCreateUser(phone string) *User {
	ms.Mu.Lock()
	defer ms.Mu.Unlock()
	if user, ok := ms.Users[phone]; ok {
		return user
	}
	newUser := &User{
		PhoneNumber: phone,
	}
	ms.Users[phone] = newUser
	return newUser
}

// GetUser Thread-safe retrieval
func (ms *MemoryStore) GetUser(phone string) *User {
	ms.Mu.RLock()
	defer ms.Mu.RUnlock()
	return ms.Users[phone]
}

// ListUsers Example: A function that lists all users (just to show RLock usage)
func (ms *MemoryStore) ListUsers() []*User {
	ms.Mu.RLock()
	defer ms.Mu.RUnlock()
	var result []*User
	for _, u := range ms.Users {
		result = append(result, u)
	}
	return result
}
