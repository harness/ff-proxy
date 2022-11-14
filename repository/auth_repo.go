package repository

import (
	"context"
	"fmt"
	"strings"
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// cleanup old unused keys for specified envs before we set the new ones
	ar.clearCachedKeys(ctx, config)

	if err := ar.Add(ctx, apiEnvMapToAuthConfig(config)...); err != nil {
		cancel()
		return AuthRepo{}, fmt.Errorf("failed to add config: %s", err)
	}
	cancel()

	return ar, nil
}

// clearCachedKeys clears any old existing keys for the specified environments we want to configure
// the reason we do this instead of RemoveAll is a user may have one proxy running against env a
// and another proxy running against env b, both connected to the same redis cache
// we only want to clear old keys related to the envs this individual proxy is configuring and not
// wipe anyone else's data
func (a AuthRepo) clearCachedKeys(ctx context.Context, newConfig map[domain.AuthAPIKey]string) {
	// get all auth keys
	currentConfig, ok := a.getAll(ctx)
	if !ok {
		// no keys exist so we can safely return
		return
	}

	// what envs are being set in new config
	envsToAdd := map[string]struct{}{}
	for _, env := range newConfig {
		envsToAdd[env] = struct{}{}
	}

	// remove all existing keys for affected envs
	for key, env := range currentConfig {
		// if env exists in envsToAdd map delete the key
		_, ok := envsToAdd[env]
		if ok {
			a.cache.Remove(ctx, AuthKey, string(key))
		}
	}
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

// getAll gets all values from auth repo
func (a AuthRepo) getAll(ctx context.Context) (map[domain.AuthAPIKey]string, bool) {

	results, err := a.cache.GetAll(ctx, AuthKey)
	if err != nil {
		return map[domain.AuthAPIKey]string{}, false
	}

	keys := map[domain.AuthAPIKey]string{}
	for key, b := range results {
		var env = strings.Trim(string(b), "\"")

		keys[domain.AuthAPIKey(key)] = env
	}

	return keys, true
}

func apiEnvMapToAuthConfig(config map[domain.AuthAPIKey]string) []domain.AuthConfig {
	authConfig := []domain.AuthConfig{}
	for key, env := range config {
		authConfig = append(authConfig, domain.AuthConfig{
			APIKey:        key,
			EnvironmentID: domain.EnvironmentID(env),
		})
	}
	return authConfig
}
