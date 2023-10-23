package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/domain"
)

// AuthRepo is a repository that stores a map of api key hashes to environmentIDs
type AuthRepo struct {
	cache                cache.Cache
	approvedEnvironments map[string]struct{}
}

// NewAuthRepo creates an AuthRepo from a map of api key hashes to environmentIDs
func NewAuthRepo(c cache.Cache) AuthRepo {
	return AuthRepo{
		cache:                c,
		approvedEnvironments: nil,
	}
}

// Add adds environment api key hash pairs to the cache
func (a AuthRepo) Add(ctx context.Context, values ...domain.AuthConfig) error {

	var key domain.APIConfigsKey
	apiKeys := make([]string, 0, len(values))

	if len(values) > 0 {
		key = domain.NewAPIConfigsKey(string(values[0].EnvironmentID))
	}

	errs := []error{}
	for i := 0; i < len(values); i++ {
		value := values[i]
		apiKeys = append(apiKeys, string(value.APIKey))
		if err := a.cache.Set(ctx, string(value.APIKey), &value.EnvironmentID); err != nil {
			errs = append(errs, addError{string(value.APIKey), string(value.APIKey), err})
		}
	}
	// set the all keys for the env
	if len(apiKeys) > 0 {
		if err := a.cache.Set(ctx, string(key), apiKeys); err != nil {
			errs = append(errs, addError{string(key), strings.Join(apiKeys, ","), err})
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to add authConfig(s) to repo: %v", errs)
	}
	return nil
}

// Get gets the environmentID for the passed api key hash
// if the auth repo has been configured with approved envs only return keys that belong to those envs
func (a AuthRepo) Get(ctx context.Context, key domain.AuthAPIKey) (string, bool) {
	var environment domain.EnvironmentID

	if err := a.cache.Get(ctx, string(key), &environment); err != nil {
		return "", false
	}

	// if we're filtering by env then check result belongs to approved env
	if len(a.approvedEnvironments) > 0 {
		if _, exists := a.approvedEnvironments[string(environment)]; !exists {
			return "", false
		}
	}

	return string(environment), true
}

// GetKeysForEnvironment gets all the apikey keys associated with environment id
func (a AuthRepo) GetKeysForEnvironment(ctx context.Context, envID string) ([]string, bool) {

	var apiKeys []string

	key := domain.NewAPIConfigsKey(envID)
	if err := a.cache.Get(ctx, string(key), &apiKeys); err != nil {
		return apiKeys, false
	}

	return apiKeys, true
}

// RemoveAllKeysForEnvironment all api keys for given environment
func (a AuthRepo) RemoveAllKeysForEnvironment(ctx context.Context, envID string) error {

	apiKeys, ok := a.GetKeysForEnvironment(ctx, envID)
	if !ok {
		return fmt.Errorf("unable to get apiKeys for environment: %v", envID)
	}

	// append the entry for the list of keys assocaited with environments
	// we do that to delete them all in the next step.
	key := domain.NewAPIConfigsKey(envID)
	apiKeys = append(apiKeys, string(key))

	//remove entries for all keys associated with environments
	return a.Remove(ctx, apiKeys)
}

// Remove removes from cache all provided keys
func (a AuthRepo) Remove(ctx context.Context, keys []string) error {

	for _, k := range keys {
		if err := a.cache.Delete(ctx, k); err != nil {
			return err
		}
	}
	return nil
}
