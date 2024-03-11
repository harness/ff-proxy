package domain

import "sync"

// SafeBool is a Bool that's safe for concurrent use
type SafeBool struct {
	*sync.RWMutex
	value bool
}

// NewSafeBool creates a SafeBool
func NewSafeBool(value bool) *SafeBool {
	return &SafeBool{
		RWMutex: &sync.RWMutex{},
		value:   value,
	}
}

// Set sets the value of the SafeBool
func (s *SafeBool) Set(v bool) {
	s.Lock()
	defer s.Unlock()

	s.value = v
}

// Get gets the value of the SafeBool
func (s *SafeBool) Get() bool {
	s.RLock()
	defer s.RUnlock()

	return s.value
}
