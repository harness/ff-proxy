package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeBool(t *testing.T) {

	b := NewSafeBool(false)
	assert.Equal(t, false, b.Get())

	b.Set(true)

	assert.Equal(t, true, b.Get())
}
