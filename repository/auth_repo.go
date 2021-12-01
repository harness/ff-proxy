package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/harness/ff-proxy/cache"
	"github.com/harness/ff-proxy/domain"
)

// AuthKey is the parent cache key we store all api key environment pairs under
const AuthKey = "auth-config"

// AuthRepo is a repository that stores a map of api key hashes to environmentIDs
type AuthRepo struct {
	cache cache.Cache
}

// NewAuthRepo creates an AuthRepo from a map of api key hashes to environmentIDs
func NewAuthRepo(c cache.Cache, config map[domain.AuthAPIKey]string) (AuthRepo, error) {
	ar := AuthRepo{cache: c}
	if config == nil || len(config) == 0 {
		return ar, nil
	}

	// cleanup old unused keys before we set the new ones
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	ar.cache.RemoveAll(ctx, AuthKey)

	if err := ar.Add(ctx, apiEnvMapToAuthConfig(config)...); err != nil {
		cancel()
		return AuthRepo{}, fmt.Errorf("failed to add config: %s", err)
	}
	cancel()

	return ar, nil
}

// Add adds environment api key hash pairs to the cache
func (a AuthRepo) Add(ctx context.Context, values ...domain.AuthConfig) error {
	errs := []error{}
	for _, v := range values {
		if err := a.cache.Set(ctx, AuthKey, string(v.APIKey), &v.EnvironmentID); err != nil {
			errs = append(errs, addErr{AuthKey, string(v.APIKey), err})
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to add authConfig(s) to repo: %v", errs)
	}
	return nil
}

// Get gets the environmentID for the passed api key hash
func (a AuthRepo) Get(ctx context.Context, key domain.AuthAPIKey) (string, bool) {
	var environment domain.EnvironmentID

	if err := a.cache.Get(ctx, AuthKey, string(key), &environment); err != nil {
		return "", false
	}

	return string(environment), true
}

func apiEnvMapToAuthConfig(config map[domain.AuthAPIKey]string) []domain.AuthConfig {
	authConfig := []domain.AuthConfig{}
	for key, env := range config {
		authConfig = append(authConfig, domain.AuthConfig{
			APIKey:  key,
			EnvironmentID: domain.EnvironmentID(env),
		})
	}
	return authConfig
}