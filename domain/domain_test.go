package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeMap(t *testing.T) {
	expectedM := map[string]interface{}{
		"foo": "bar",
	}

	m := NewSafeMap()

	m.Set("foo", "bar")
	actualM := m.Get()

	assert.Equal(t, expectedM, actualM)
}
