package domain

import (
	"reflect"
	"testing"

	jsoniter "github.com/json-iterator/go"
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

func TestStreamState_String(t *testing.T) {
	assert.Equal(t, "CONNECTED", StreamStateConnected.String())
	assert.Equal(t, "DISCONNECTED", StreamStateDisconnected.String())
	assert.Equal(t, "INITIALIZING", StreamStateInitializing.String())
}

func TestNewAuthAPIKey(t *testing.T) {
	expected := AuthAPIKey("auth-key-123")
	actual := NewAuthAPIKey("123")

	assert.Equal(t, expected, actual)
}

func TestEnvironmentID_MarshalBinary(t *testing.T) {
	envID := EnvironmentID("123")
	expected, err := jsoniter.Marshal(envID)
	assert.Nil(t, err)

	actual, err := envID.MarshalBinary()
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestEnvironmentID_UnmarshalBinary(t *testing.T) {
	envID := EnvironmentID("123")
	b, err := jsoniter.Marshal(envID)
	assert.Nil(t, err)

	var expected EnvironmentID
	assert.Nil(t, jsoniter.Unmarshal(b, &expected))

	var actual EnvironmentID
	assert.Nil(t, actual.UnmarshalBinary(b))

	assert.Equal(t, expected, actual)
}

func TestNewConfigStatus(t *testing.T) {
	expected := ConfigStatus{State: ConfigStateSynced}

	actual := NewConfigStatus(ConfigStateSynced)

	assert.Equal(t, expected.State, actual.State)
	assert.NotEqual(t, int64(0), actual.Since)
}

func TestStreamStatus_MarshalBinary(t *testing.T) {
	streamStatus := NewStreamStatus()
	expected, err := jsoniter.Marshal(streamStatus)
	assert.Nil(t, err)

	actual, err := streamStatus.MarshalBinary()
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

func TestStreamStatus_UnmarshalBinary(t *testing.T) {
	streamStatus := NewStreamStatus()
	b, err := jsoniter.Marshal(streamStatus)
	assert.Nil(t, err)

	var expected StreamStatus
	assert.Nil(t, jsoniter.Unmarshal(b, &expected))

	var actual StreamStatus
	assert.Nil(t, actual.UnmarshalBinary(b))

	assert.Equal(t, expected, actual)
}

func Test_ToPtr(t *testing.T) {
	s := "foo"
	actual := ToPtr(s)
	assert.True(t, reflect.ValueOf(actual).Kind() == reflect.Ptr)
}
