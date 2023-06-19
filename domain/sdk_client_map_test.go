package domain

import (
	"testing"

	harness "github.com/harness/ff-golang-server-sdk/client"
	"github.com/stretchr/testify/assert"
)

func TestNewSDKClientMap(t *testing.T) {
	actual := NewSDKClientMap()
	assert.NotNil(t, actual)
	assert.NotNil(t, actual.RWMutex)
	assert.NotNil(t, actual.m)
}

func TestSDKClientMap_Set(t *testing.T) {
	cm := NewSDKClientMap()

	cfClient := &harness.CfClient{}

	cm.Set("foo", cfClient)

	actual, ok := cm.m["foo"]
	assert.True(t, ok)

	assert.Equal(t, *cfClient, *actual)
}

func TestSDKClientMap_Copy(t *testing.T) {
	cm := NewSDKClientMap()

	cfClient := &harness.CfClient{}
	cm.Set("foo", cfClient)

	expected := map[string]*harness.CfClient{
		"foo": cfClient,
	}

	actual := cm.Copy()

	assert.Equal(t, expected, actual)
}

func TestSDKClientMap_StreamConnected(t *testing.T) {
	cm := NewSDKClientMap()

	cfClient := &harness.CfClient{}
	cm.Set("foo", cfClient)

	const expected = false
	actual := cm.StreamConnected("foo")

	assert.Equal(t, expected, actual)
}
