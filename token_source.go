package ffproxy

import (
	"context"
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/log"
)

type authRepo interface {
	Get(context context.Context, key domain.AuthAPIKey) (string, bool)
}

type hasher interface {
	Hash(s string) string
}

// TokenSource is a type that can create and validate tokens
type TokenSource struct {
	repo   authRepo
	hasher hasher
	secret []byte
	log    log.Logger
}

// NewTokenSource creates a new TokenSource
func NewTokenSource(l log.Logger, repo authRepo, hasher hasher, secret []byte) TokenSource {
	l = log.With(l, "component", "TokenSource")
	return TokenSource{log: l, repo: repo, hasher: hasher, secret: secret}
}

// GenerateToken creates a token from a key
func (a TokenSource) GenerateToken(key string) (domain.Token, error) {
	h := a.hasher.Hash(key)

	env, ok := a.repo.Get(context.Background(), domain.AuthAPIKey(h))
	if !ok {
		return domain.Token{}, fmt.Errorf("Key %q not found", key)
	}

	t := time.Now().Unix()
	c := domain.Claims{
		Environment: env,
		StandardClaims: jwt.StandardClaims{
			IssuedAt:  t,
			NotBefore: t,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	authToken, err := token.SignedString(a.secret)
	if err != nil {
		return domain.Token{}, err
	}

	return domain.NewToken(authToken, c), nil
}
