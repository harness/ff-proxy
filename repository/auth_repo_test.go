package repository

import (
	"context"
	"testing"

	"github.com/harness/ff-proxy/v2/cache"

	"github.com/stretchr/testify/assert"

	"github.com/harness/ff-proxy/v2/domain"
)

func TestAuthRepo_Get(t *testing.T) {
	populated := []domain.AuthConfig{
		{
			APIKey:        domain.AuthAPIKey("apikey-foo"),
			EnvironmentID: domain.EnvironmentID("env-approved"),
		},
		{
			APIKey:        domain.AuthAPIKey("apikey-2"),
			EnvironmentID: domain.EnvironmentID("env-not-approved"),
		},
	}
	unpopulated := []domain.AuthConfig{}

	type expected struct {
		strVal  string
		boolVal bool
	}

	testCases := map[string]struct {
		cache    cache.Cache
		data     []domain.AuthConfig
		key      string
		expected expected
	}{
		"Given I have an empty AuthRepo": {
			cache:    cache.NewMemCache(),
			data:     unpopulated,
			key:      "apikey-foo",
			expected: expected{strVal: "", boolVal: false},
		},
		"Given I have a populated AuthRepo but try to get a key that doesn't exist": {
			cache:    cache.NewMemCache(),
			data:     populated,
			key:      "foo",
			expected: expected{strVal: "", boolVal: false},
		},
		"Given I have a populated AuthRepo and try to get a key that does exist": {
			cache:    cache.NewMemCache(),
			data:     populated,
			key:      "apikey-foo",
			expected: expected{strVal: "env-approved", boolVal: true},
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			ctx := context.Background()
			repo := NewAuthRepo(tc.cache)

			assert.Nil(t, repo.Add(ctx, tc.data...))

			actual, ok := repo.Get(ctx, domain.AuthAPIKey(tc.key))

			assert.Equal(t, tc.expected.boolVal, ok)
			assert.Equal(t, tc.expected.strVal, actual)
		})
	}
}

func TestAPIRepo_Remove(t *testing.T) {
	populatedConfig := []domain.AuthConfig{
		{
			APIKey:        domain.AuthAPIKey("apikey-foo"),
			EnvironmentID: domain.EnvironmentID("env-approved"),
		},
		{
			APIKey:        domain.AuthAPIKey("apikey-2"),
			EnvironmentID: domain.EnvironmentID("env-not-approved"),
		},
	}
	emptyConfig := []domain.AuthConfig{}

	testCases := map[string]struct {
		cache      cache.MemCache
		repoConfig []domain.AuthConfig
		shouldErr  bool
	}{
		"Given I call Remove with and the ApiKey config does not exist": {
			cache:      cache.NewMemCache(),
			repoConfig: emptyConfig,
			shouldErr:  true,
		},
		"Given I call Remove with and the ApiKey config does exist": {
			cache:      cache.NewMemCache(),
			repoConfig: populatedConfig,
			shouldErr:  false,
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			ctx := context.Background()
			repo := NewAuthRepo(tc.cache)

			if tc.shouldErr {
				assert.Error(t, repo.RemoveAllKeysForEnvironment(ctx, "env-approved"))

			} else {
				assert.Nil(t, repo.Add(ctx, tc.repoConfig...))
				assert.Nil(t, repo.RemoveAllKeysForEnvironment(ctx, "env-approved"))

			}
		})
	}
}
