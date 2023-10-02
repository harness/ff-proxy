package repository

import (
	"fmt"
)

// addError is used for formatting errors that occur when adding a value to a repo
type addError struct {
	key        string
	identifier string
	err        error
}

// Error makes addError implement the error interface
func (a addError) Error() string {
	return fmt.Sprintf("failed to add target - key: %s, identifier: %s, err: %s", a.key, a.identifier, a.err)
}
