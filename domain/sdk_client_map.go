package domain

import (
	"sync"

	harness "github.com/harness/ff-golang-server-sdk/client"
)

// SDKClientMap is a map of environmentIDs to sdks
type SDKClientMap struct {
	*sync.RWMutex
	m map[string]*harness.CfClient
}

// NewSDKClientMap creates an SDKClientMap
func NewSDKClientMap() *SDKClientMap {
	return &SDKClientMap{
		RWMutex: &sync.RWMutex{},
		m:       map[string]*harness.CfClient{},
	}
}

// Set sets a key and value in the map
func (s *SDKClientMap) Set(key string, value *harness.CfClient) {
	s.Lock()
	defer s.Unlock()
	s.m[key] = value
}

// Copy returns a copy of the map
func (s *SDKClientMap) Copy() map[string]*harness.CfClient {
	s.RLock()
	defer s.RUnlock()
	return s.m
}

// StreamConnected checks if the sdk for the given key has a healthy stream connection.
// If an SDK doesn't exist for the key it will return false
func (s *SDKClientMap) StreamConnected(key string) bool {
	s.Lock()
	defer s.Unlock()

	sdk, ok := s.m[key]
	if !ok {
		return false
	}
	return sdk.IsStreamConnected()
}
