package token

import (
	"context"
	"testing"
	"time"

	"github.com/harness/ff-proxy/v2/hash"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/log"

	"github.com/golang-jwt/jwt/v4"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/repository"
	"github.com/stretchr/testify/assert"
)

func TestTokenSource_GenerateToken(t *testing.T) {
	const (
		unhashedKey = "21ee6c7a-f78d-4afd-86a1-5c108aad41e8"
		hashedKey   = "bc1ca6b8271bfef0485c9f2978cbb2e1536801f312dc069a344c85146ad7cdb3"
		envID       = "aba48e5a-3161-4622-b4c4-a3fcc2f22ed7"
	)
	secret := []byte(`secret`)

	authConfig := domain.AuthConfig{
		APIKey:        domain.NewAuthAPIKey(hashedKey),
		EnvironmentID: envID,
	}

	authRepo := repository.NewAuthRepo(cache.NewMemCache())
	assert.Nil(t, authRepo.Add(context.Background(), authConfig))

	tokenSource := NewTokenSource(log.NoOpLogger{}, authRepo, hash.NewSha256(), secret)

	testCases := map[string]struct {
		key         string
		shouldErr   bool
		expectedEnv string
	}{
		"Given I call GenerateToken with an empty key": {
			key:         "",
			shouldErr:   true,
			expectedEnv: "",
		},
		"Given I call GenerateToken with a key that isn't in the repo": {
			key:         "foobar",
			shouldErr:   true,
			expectedEnv: "",
		},
		"Given I call GenerateToken with a key that is in the repo": {
			key:         unhashedKey,
			shouldErr:   false,
			expectedEnv: envID,
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {

			actual, err := tokenSource.GenerateToken(tc.key)
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			if tc.expectedEnv != "" {
				token, err := jwt.ParseWithClaims(actual.TokenString(), &domain.Claims{}, func(t *jwt.Token) (interface{}, error) {
					return secret, nil
				})
				if err != nil {
					t.Fatal(err)
				}

				claims, ok := token.Claims.(*domain.Claims)
				assert.True(t, ok)
				assert.Equal(t, tc.expectedEnv, claims.Environment)
				assert.Nil(t, claims.Valid())
			} else {
				assert.Equal(t, tc.expectedEnv, actual.Claims().Environment)
			}

		})
	}
}

func mustGenerateFakeToken(t *testing.T, secret []byte) string {
	type fakeClaims struct {
		Foobar string
		jwt.StandardClaims
	}

	now := time.Now().Unix()
	claims := fakeClaims{
		Foobar: "hello",
		StandardClaims: jwt.StandardClaims{
			IssuedAt:  now,
			NotBefore: now,
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := tok.SignedString(secret)
	if err != nil {
		t.Fatal(err)
	}

	return token
}
