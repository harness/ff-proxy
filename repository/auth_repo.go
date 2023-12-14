package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/harness/ff-proxy/cache"
	"github.com/harness/ff-proxy/domain"
)

// AuthRepo is a repository that stores a map of api key hashes to environmentIDs
type AuthRepo struct {
	cache                cache.Cache
	approvedEnvironments map[string]struct{}
}

// NewAuthRepo creates an AuthRepo from a map of api key hashes to environmentIDs
func NewAuthRepo(c cache.Cache, config map[domain.AuthAPIKey]string, approvedEnvironments map[string]struct{}) (AuthRepo, error) {
	ar := AuthRepo{cache: c, approvedEnvironments: approvedEnvironments}
	if config == nil || len(config) == 0 {
		return ar, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
			a.cache.Delete(ctx, string(key))
		}
	}
}

// Add adds environment api key hash pairs to the cache
func (a AuthRepo) Add(ctx context.Context, values ...domain.AuthConfig) error {
	errs := []error{}
	for i := 0; i < len(values); i++ {
		value := values[i]
		if err := a.cache.Set(ctx, string(value.APIKey), &value.EnvironmentID); err != nil {
			errs = append(errs, addErr{string(value.APIKey), string(value.APIKey), err})
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to add authConfig(s) to repo: %v", errs)
	}
	return nil
}

// Get gets the environmentID for the passed api key hash
// if the auth repo has been configured with approved envs only return keys that belong to those envs
func (a AuthRepo) Get(ctx context.Context, key domain.AuthAPIKey) (string, error) {
	var environment domain.EnvironmentID

	if err := a.cache.Get(ctx, string(key), &environment); err != nil {
		return "", err
	}

	// if we're filtering by env then check result belongs to approved env
	if len(a.approvedEnvironments) > 0 {
		if _, exists := a.approvedEnvironments[string(environment)]; !exists {
			return "", fmt.Errorf("%w: unapproved env", domain.ErrCacheInternal)
		}
	}

	return string(environment), nil
}

// getAll gets all values from auth repo
// if the auth repo has been configured with approved envs only return keys that belong to those envs
func (a AuthRepo) getAll(ctx context.Context) (map[domain.AuthAPIKey]string, bool) {
	keys, err := a.cache.Keys(ctx, "auth-key-")
	if err != nil {
		return map[domain.AuthAPIKey]string{}, false
	}

	if len(keys) == 0 {
		return map[domain.AuthAPIKey]string{}, false
	}

	results := map[domain.AuthAPIKey]string{}
	numApprovedEnvs := len(a.approvedEnvironments)

	for _, key := range keys {
		environment := ""
		if err := a.cache.Get(ctx, key, &environment); err != nil {
			continue
		}

		// Skip if this environment isn't in our list of approved envs
		if numApprovedEnvs > 0 {
			if _, exists := a.approvedEnvironments[environment]; !exists {
				continue
			}
		}

		results[domain.AuthAPIKey(key)] = environment
	}

	if len(results) == 0 {
		return results, false
	}
	return results, true
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
