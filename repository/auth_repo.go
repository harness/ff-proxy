package repository

import (
	"sync"

	"github.com/harness/ff-proxy/domain"
)

// AuthRepo is a repository that stores a map of api key hashes to environmentIDs
type AuthRepo struct {
	*sync.RWMutex
	data map[domain.AuthAPIKey]string
}

// NewAuthRepo creates an AuthRepo from a map of api key hashes to environmentIDs
func NewAuthRepo(data map[domain.AuthAPIKey]string) AuthRepo {
	if data == nil {
		data = make(map[domain.AuthAPIKey]string)
	}
	return AuthRepo{&sync.RWMutex{}, data}
}

// Get gets the environmentID for the passed api key hash
func (a AuthRepo) Get(key domain.AuthAPIKey) (string, bool) {
	a.RLock()
	defer a.RUnlock()
	v, ok := a.data[key]
	return v, ok
}
