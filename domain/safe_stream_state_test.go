package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSafeStreamState(t *testing.T) {
	s := NewSafeStreamStatus(NewStreamStatus())

	expected := StreamStatus{State: StreamStateConnected, Since: time.Now().UnixMilli()}

	s.Set(expected)

	actual := s.Get()
	assert.Equal(t, expected, actual)
}
