package repository

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/exp/slices"

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
	apiKeys := make([]string, 0, len(values))
	errs := []error{}
	for i := 0; i < len(values); i++ {
		value := values[i]
		apiKeys = append(apiKeys, string(value.APIKey))
		if err := a.cache.Set(ctx, string(value.APIKey), &value.EnvironmentID); err != nil {
			errs = append(errs, addError{string(value.APIKey), string(value.APIKey), err})
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to add authConfig(s) to repo: %v", errs)
	}
	return nil
}

// AddAPIConfigsForEnvironment adds/overrides the list of api keys on populate.
func (a AuthRepo) AddAPIConfigsForEnvironment(ctx context.Context, envID string, apiKeys []string) error {
	key := domain.NewAPIConfigsKey(envID)
	return a.cache.Set(ctx, string(key), apiKeys)
}

// Get gets the environmentID for the passed api key hash
// if the auth repo has been configured with approved envs only return keys that belong to those envs
func (a AuthRepo) Get(ctx context.Context, key domain.AuthAPIKey) (string, bool, error) {
	var environment domain.EnvironmentID

	if err := a.cache.Get(ctx, string(key), &environment); err != nil {
		return "", false, err
	}

	// if we're filtering by env then check result belongs to approved env
	if len(a.approvedEnvironments) > 0 {
		if _, exists := a.approvedEnvironments[string(environment)]; !exists {
			return "", false, nil
		}
	}

	return string(environment), true, nil
}

// GetKeysForEnvironment gets all the apikey keys associated with environment id
func (a AuthRepo) GetKeysForEnvironment(ctx context.Context, envID string) ([]string, error) {

	var apiKeys []string

	key := domain.NewAPIConfigsKey(envID)
	if err := a.cache.Get(ctx, string(key), &apiKeys); err != nil {
		return apiKeys, err
	}

	return apiKeys, nil
}

// RemoveAllKeysForEnvironment all api keys for given environment
func (a AuthRepo) RemoveAllKeysForEnvironment(ctx context.Context, envID string) error {

	apiKeys, err := a.GetKeysForEnvironment(ctx, envID)
	if err != nil {
		return fmt.Errorf("unable to get apiKeys for environment %s: %w", envID, err)
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

// PatchAPIConfigForEnvironment Updates the list of keys for given environment
func (a AuthRepo) PatchAPIConfigForEnvironment(ctx context.Context, envID, key, action string) error {
	apiKey := string(domain.NewAuthAPIKey(key))
	apiConfigsKey := domain.NewAPIConfigsKey(envID)
	apiConfigsValue, err := a.GetKeysForEnvironment(ctx, envID)
	if err != nil {
		if !errors.Is(err, domain.ErrCacheNotFound) {
			return err
		}
	}

	switch action {
	case domain.EventAPIKeyAdded:
		// 1. Environment config does not exist - excellent, create apiConfigsKey entry
		apiConfigsValue, done, err := a.handleAdd(ctx, apiConfigsValue, apiKey, apiConfigsKey)
		if done {
			return err
		}
		return a.cache.Set(ctx, string(apiConfigsKey), apiConfigsValue)

	case domain.EventAPIKeyRemoved:
		//1. Environment config does not exit - do nothing
		newAPIKeys, done, err := a.handleRemove(ctx, apiConfigsValue, apiKey, apiConfigsKey)
		if done {
			return err
		}
		return a.cache.Set(ctx, string(apiConfigsKey), newAPIKeys)
	}
	return fmt.Errorf("action %v is not permitted", action)
}

func (a AuthRepo) handleRemove(ctx context.Context, apiConfigsValue []string, apiKey string, apiConfigsKey domain.APIConfigsKey) ([]string, bool, error) {
	if len(apiConfigsValue) < 1 {
		return nil, true, nil
	}
	// 2. Environment config does exist but does not contain the target_key - do nothing
	if len(apiConfigsValue) > 0 && !slices.Contains(apiConfigsValue, apiKey) {
		return nil, true, nil
	}
	// 3. Environment config does exist and only contains key to remove - delete apiConfigsKey entry
	if len(apiConfigsValue) == 1 && slices.Contains(apiConfigsValue, apiKey) {
		return nil, true, a.cache.Delete(ctx, string(apiConfigsKey))
	}
	//4. Environment config does exist and contains key to remove - remove the key entry and reset apiConfigsKey
	newAPIKeys := make([]string, 0, len(apiConfigsValue)-1)
	for _, v := range apiConfigsValue {
		if v != apiKey {
			newAPIKeys = append(newAPIKeys, v)
		}
	}
	return newAPIKeys, false, nil
}

func (a AuthRepo) handleAdd(ctx context.Context, apiConfigsValue []string, apiKey string, apiConfigsKey domain.APIConfigsKey) ([]string, bool, error) {
	if len(apiConfigsValue) < 1 {
		apiConfigsValue = append(apiConfigsValue, apiKey)
		return nil, true, a.cache.Set(ctx, string(apiConfigsKey), apiConfigsValue)
	}
	// 2. Environment configs exist but already contains the target_key - do nothing
	if slices.Contains(apiConfigsValue, apiKey) {
		return nil, true, nil
	}
	// 3. Environment config exits and does not contain the keys - add key
	apiConfigsValue = append(apiConfigsValue, apiKey)
	return apiConfigsValue, false, nil
}
