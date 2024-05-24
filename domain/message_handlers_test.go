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

func TestReadReplicaMessageHandler_HandleMessage(t *testing.T) {

	type args struct {
		msg SSEMessage
	}

	type mocks struct {
		health *mockHealth
	}

	type expected struct {
		health bool
		err    error
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
			},
			expected: expected{
				health: false,
				err:    nil,
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
			},
			expected: expected{
				health: true,
				err:    nil,
			},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			ctx := context.Background()

			r := NewReadReplicaMessageHandler(log.NoOpLogger{}, tc.mocks.health)

			err := r.HandleMessage(ctx, tc.args.msg)
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.health, tc.mocks.health.getHealth())
		})
	}
}
