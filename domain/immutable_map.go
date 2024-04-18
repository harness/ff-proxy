package domain

// ImmutableSet is a struct representing an immutable set
type ImmutableSet struct {
	data map[string]struct{}
}

// NewImmutableSet creates a new ImmutableSet with the given data.
func NewImmutableSet(data map[string]struct{}) *ImmutableSet {
	return &ImmutableSet{data: data}
}

// Has checks if the key exists in the Set
func (i *ImmutableSet) Has(key string) bool {
	_, ok := i.data[key]
	return ok
}
