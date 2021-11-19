package repository

import (
	"fmt"
)

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
