package repository

import (
	"context"
	"encoding"
	"fmt"
)

// Cache is the interface for any type that stores keys against a map of fields -> values
// e.g.
//
// some-key-1
//    field-1: foobar
//    field-2: fizzbuzz
// some-key-2
//    field-1: hello-world
type Cache interface {
	// Set sets a value in the cache for a given key and field
	Set(ctx context.Context, key string, field string, value encoding.BinaryMarshaler) error
	// Get gets the value of a field for a given key
	Get(ctx context.Context, key string, field string, v encoding.BinaryUnmarshaler) error
	// GetAll gets all of the fiels and their values for a given key
	GetAll(ctx context.Context, key string) (map[string][]byte, error)
}

// addErr is used for formatting errors that occur when adding a value to a repo
type addErr struct {
	key        string
	identifier string
	err        error
}

// Error makes addErr implement the error interface
func (a addErr) Error() string {
	return fmt.Sprintf("failed to add target - key: %s, identifier: %s, err: %s", a.key, a.identifier, a.err)
}
