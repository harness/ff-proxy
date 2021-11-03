package domain

import "fmt"

// CacheOperation is the type of operation performed on the cache. It's used
// with the CacheNotFoundErr and CacheInternalErr types to help provide descriptive
// error messages
type CacheOperation string

const (
	// CacheOpSet is the name for the Set operation
	CacheOpSet CacheOperation = "Set"

	// CacheOpGetAll is the name for the GetAll operation
	CacheOpGetAll = "GetAll"

	// CacheOpGet is the name for the Get operation
	CacheOpGet = "Get"
)

// CacheNotFoundErr is the error returned by a cache when there is no value for
// a Key/Field
type CacheNotFoundErr struct {
	err          error
	orginalError error
}

// NewCacheNotFoundErr creates a new CacheNotFoundErr
func NewCacheNotFoundErr(op CacheOperation, key, field string, err error) CacheNotFoundErr {
	return CacheNotFoundErr{
		orginalError: err,
		err:          fmt.Errorf("operation: %v, no value found in cache for key: %s, field: %s: %w", op, key, field, err),
	}
}

// Error makes CacheNotFoundErr implement the error interface
func (c CacheNotFoundErr) Error() string {
	return c.err.Error()
}

// CacheInternalErr is the error returned by a cache when there is an unexpected error
type CacheInternalErr struct {
	err          error
	orginalError error
}

// NewCacheInternalErr creates a new CacheInternalErr
func NewCacheInternalErr(op CacheOperation, key, field string, err error) CacheNotFoundErr {
	return CacheNotFoundErr{
		orginalError: err,
		err:          fmt.Errorf("operation: %v, unexpected error occured in cache for key: %s, field: %s: %w", op, key, field, err),
	}
}

// Error makes CacheInternalErr implement the error interface
func (c CacheInternalErr) Error() string {
	return c.err.Error()
}
