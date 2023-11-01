package domain

import (
	"fmt"
)

// KeyInventory maps all assets associated with for proxy key.
type KeyInventory string

// NewKeyInventory creates a key inventory entry for the proxy key. This key contains all the entries associated with the proxy key.
func NewKeyInventory(key string) KeyInventory {
	return KeyInventory(fmt.Sprintf("key-%s-inventory", key))
}
