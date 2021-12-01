package repository

import (
	"context"
	"github.com/harness/ff-proxy/cache"
	"testing"

	"github.com/harness/ff-proxy/domain"
	"github.com/stretchr/testify/assert"
)

func TestAuthRepo_Get(t *testing.T) {
	populated := map[domain.AuthAPIKey]string{
		domain.AuthAPIKey("apikey-foo"): "env-bar",
	}
	unpopulated := map[domain.AuthAPIKey]string{}

	type expected struct {
		strVal  string
		boolVal bool
	}

	testCases := map[string]struct {
		cache    cache.Cache
		data     map[domain.AuthAPIKey]string
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
			expected: expected{strVal: "env-bar", boolVal: true},
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {

			repo, err := NewAuthRepo(tc.cache, tc.data)
			if err != nil {
				t.Fatalf("(%s): error = %v", desc, err)
			}
			actual, ok := repo.Get(context.Background(), domain.AuthAPIKey(tc.key))

			assert.Equal(t, tc.expected.boolVal, ok)
			assert.Equal(t, tc.expected.strVal, actual)
		})
	}
}
