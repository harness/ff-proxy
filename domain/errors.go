package domain

import (
	"errors"
)

var (
	// ErrCacheNotFound is the error returned by a cache when there is no value for
	// a Key/Field
	ErrCacheNotFound = errors.New("cache: not found")

	// ErrCacheInternal is the error returned by a cache when there is an unexpected error
	ErrCacheInternal = errors.New("cache: internal error")

	// ErrConnRefused is the error returned by a cache when we can't connect to it
	ErrConnRefused = errors.New("cache: connection refused")
)
