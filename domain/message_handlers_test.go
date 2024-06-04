package domain

import (
	"context"
	"sync"
	"testing"

	"github.com/harness/ff-proxy/v2/log"
	"github.com/stretchr/testify/assert"
)

type mockHealth struct {
	*sync.Mutex
	healthy bool
}

func (m *mockHealth) SetUnhealthy(ctx context.Context) error {
	m.Lock()
	defer m.Unlock()
	m.healthy = false

	return nil
}

func (m *mockHealth) SetHealthy(ctx context.Context) error {
	m.Lock()
	defer m.Unlock()
	m.healthy = true

	return nil
}

func (m *mockHealth) getHealth() bool {
	m.Lock()
	defer m.Unlock()
	return m.healthy
}

type mockPushpin struct {
	*sync.Mutex
	closed bool
}

func (m *mockPushpin) Close(topic string) error {
	m.Lock()
	defer m.Unlock()

	m.closed = true
	return nil
}

func (m *mockPushpin) status() bool {
	m.Lock()
	defer m.Unlock()

	return m.closed
}

func TestReadReplicaMessageHandler_HandleMessage(t *testing.T) {
	connectedStreams := func() map[string]interface{} {
		return map[string]interface{}{
			"env-123": struct{}{},
		}
	}

	type args struct {
		msg SSEMessage
	}

	type mocks struct {
		health           *mockHealth
		pp               *mockPushpin
		connectedStreams func() map[string]interface{}
	}

	type expected struct {
		health              bool
		err                 error
		pushpinStreamClosed bool
	}

	testCases := map[string]struct {
		args      args
		mocks     mocks
		expected  expected
		shouldErr bool
	}{
		"Given I have a healthy status and get a stream disconnect event": {
			args: args{
				msg: SSEMessage{
					Event:  "stream_action",
					Domain: StreamStateDisconnected.String(),
				},
			},
			mocks: mocks{
				health: &mockHealth{
					Mutex:   &sync.Mutex{},
					healthy: true,
				},
				connectedStreams: connectedStreams,
				pp:               &mockPushpin{Mutex: &sync.Mutex{}},
			},
			expected: expected{
				health:              false,
				err:                 nil,
				pushpinStreamClosed: true,
			},
			shouldErr: false,
		},
		"Given I have a unhealthy status and get a stream connect event": {
			args: args{
				msg: SSEMessage{
					Event:  "stream_action",
					Domain: StreamStateConnected.String(),
				},
			},
			mocks: mocks{
				health: &mockHealth{
					Mutex:   &sync.Mutex{},
					healthy: false,
				},
				connectedStreams: connectedStreams,
				pp:               &mockPushpin{Mutex: &sync.Mutex{}},
			},
			expected: expected{
				health:              true,
				err:                 nil,
				pushpinStreamClosed: false,
			},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			ctx := context.Background()

			r := NewReadReplicaMessageHandler(log.NoOpLogger{}, tc.mocks.health, tc.mocks.connectedStreams, tc.mocks.pp)

			err := r.HandleMessage(ctx, tc.args.msg)
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.health, tc.mocks.health.getHealth())
			assert.Equal(t, tc.expected.pushpinStreamClosed, tc.mocks.pp.status())
		})
	}
}
