package token

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

type authRepo interface {
	Get(context context.Context, key domain.AuthAPIKey) (string, bool, error)
}

type hasher interface {
	Hash(s string) string
}

// Source is a type that can create and validate tokens
type Source struct {
	repo   authRepo
	hasher hasher
	secret []byte
	log    log.Logger
}

// NewSource creates a new Source
func NewSource(l log.Logger, repo authRepo, hasher hasher, secret []byte) Source {
	l = l.With("component", "Source")
	return Source{log: l, repo: repo, hasher: hasher, secret: secret}
}

// GenerateToken creates a token from a key
func (a Source) GenerateToken(key string) (domain.Token, error) {
	h := a.hasher.Hash(key)

	k := domain.NewAuthAPIKey(h)

	env, ok, err := a.repo.Get(context.Background(), k)
	if err != nil {
		a.log.Error("failed to get auth key from cache to generate token", "err", err)
	}
	if !ok {
		return domain.Token{}, fmt.Errorf("%w: key not found", err)
	}

	t := time.Now()
	c := domain.Claims{
		APIKey:            string(k),
		Environment:       env,
		ClusterIdentifier: "1",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(t),
			NotBefore: jwt.NewNumericDate(t),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	authToken, err := token.SignedString(a.secret)
	if err != nil {
		return domain.Token{}, err
	}

	return domain.NewToken(authToken, c), nil
}
