package domain

import "github.com/golang-jwt/jwt/v4"

// Token is a type that contains a generated token string and the claims
type Token struct {
	token  string
	claims Claims
}

// NewToken creates a new token
func NewToken(tokenString string, claims Claims) Token {
	return Token{token: tokenString, claims: claims}
}

// TokenString returns the auth token string
func (t Token) TokenString() string {
	return t.token
}

// Claims returns the tokens claims
func (t Token) Claims() Claims {
	return t.claims
}

// Claims are custom jwt claims used by the proxy for generating a jwt token
type Claims struct {
	Environment       string `json:"environment"`
	ClusterIdentifier string `json:"clusterIdentifier"`
	jwt.RegisteredClaims
}
