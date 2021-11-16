package repository

import (
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
		data     map[domain.AuthAPIKey]string
		key      string
		expected expected
	}{
		"Given I have an empty AuthRepo": {
			data:     unpopulated,
			key:      "apikey-foo",
			expected: expected{strVal: "", boolVal: false},
		},
		"Given I have a populated AuthRepo but try to get a key that doesn't exist": {
			data:     populated,
			key:      "foo",
			expected: expected{strVal: "", boolVal: false},
		},
		"Given I have a populated AuthRepo and try to get a key that does exist": {
			data:     populated,
			key:      "apikey-foo",
			expected: expected{strVal: "env-bar", boolVal: true},
		},
	}
	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {

			repo := NewAuthRepo(tc.data)
			actual, ok := repo.Get(domain.AuthAPIKey(tc.key))

			assert.Equal(t, tc.expected.boolVal, ok)
			assert.Equal(t, tc.expected.strVal, actual)
		})
	}
}
