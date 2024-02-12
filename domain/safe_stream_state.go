package domain

import "sync"

// SafeStreamStatus is a StreamStatus that's safe for concurrent use
type SafeStreamStatus struct {
	*sync.RWMutex
	value StreamStatus
}

// NewSafeStreamStatus creates a SafeStreamStatus
func NewSafeStreamStatus(v StreamStatus) *SafeStreamStatus {
	return &SafeStreamStatus{
		RWMutex: &sync.RWMutex{},
		value:   v,
	}
}

// Set sets the StreamStatus
func (s *SafeStreamStatus) Set(v StreamStatus) {
	s.Lock()
	defer s.Unlock()

	s.value = v
}

// Get gets the StreamStatus
func (s *SafeStreamStatus) Get() StreamStatus {
	s.RLock()
	defer s.RUnlock()

	return s.value
}
