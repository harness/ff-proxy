package stream

import (
	"context"
	"sync"
	"testing"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/prometheus/client_golang/prometheus"
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

func (m *mockHealth) Status(ctx context.Context) (domain.StreamStatus, error) {
	return domain.StreamStatus{}, nil
}

func (m *mockHealth) getHealth() bool {
	m.Lock()
	defer m.Unlock()
	return m.healthy
}

type mockStream struct {
	events []interface{}
}

func (m *mockStream) Pub(ctx context.Context, channel string, msg interface{}) error {
	m.events = append(m.events, msg)
	return nil
}

func (m *mockStream) Sub(ctx context.Context, channel string, id string, msg domain.HandleMessageFn) error {
	m.events = append(m.events, msg)
	return nil
}

func (m *mockStream) Close(channel string) error {
	return nil
}

func TestSaasStreamOnDisconnect(t *testing.T) {
	type mocks struct {
		health  *mockHealth
		pushpin Pushpin
		stream  *mockStream

		connectedStreamsFunc func() map[string]interface{}
		pollFn               func() error
	}

	type expected struct {
		events       []interface{}
		streamHealth bool
	}

	testCases := map[string]struct {
		mocks    mocks
		expected expected
	}{
		"Given I have a healthy streams status and disconnect from the Saas stream": {
			mocks: mocks{
				health: &mockHealth{
					Mutex:   &sync.Mutex{},
					healthy: true,
				},
				pushpin: Pushpin{stream: &mockGripStream{}},
				stream:  &mockStream{events: []interface{}{}},
				pollFn: func() error {
					return nil
				},
				connectedStreamsFunc: func() map[string]interface{} {
					return map[string]interface{}{"foo": struct{}{}}
				},
			},
			expected: expected{
				events: []interface{}{
					domain.SSEMessage{Event: "stream_action", Domain: "disconnect", Identifier: "", Version: 0, Environment: "", Environments: []string(nil), APIKey: ""},
				},
				streamHealth: false,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			redisStream := NewStream(
				log.NoOpLogger{},
				"foo",
				tc.mocks.stream,
				domain.NoOpMessageHandler{},
			)

			ps := NewPollingStatusMetric(prometheus.NewRegistry())

			SaasStreamOnDisconnect(log.NoOpLogger{}, tc.mocks.health, tc.mocks.pushpin, redisStream, tc.mocks.connectedStreamsFunc, tc.mocks.pollFn, ps)()

			t.Log("Then the stream status will become unhealthy")
			assert.Equal(t, tc.expected.streamHealth, tc.mocks.health.getHealth())

			t.Log("And a disconnect event will be sent down the redis stream")
			assert.Equal(t, tc.expected.events, tc.mocks.stream.events)
		})
	}
}

func TestSaasStreamOnConnect(t *testing.T) {
	type mocks struct {
		health *mockHealth
		stream *mockStream

		reloadConfig func() error
	}

	type expected struct {
		events       []interface{}
		streamHealth bool
	}

	testCases := map[string]struct {
		mocks    mocks
		expected expected
	}{
		"Given I have a unhealthy streams status and disconnect from the Saas stream": {
			mocks: mocks{
				health: &mockHealth{
					Mutex:   &sync.Mutex{},
					healthy: false,
				},
				stream: &mockStream{events: []interface{}{}},
				reloadConfig: func() error {
					return nil
				},
			},
			expected: expected{
				events: []interface{}{
					domain.SSEMessage{Event: "stream_action", Domain: "connect", Identifier: "", Version: 0, Environment: "", Environments: []string(nil), APIKey: ""},
				},
				streamHealth: true,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			redisStream := NewStream(
				log.NoOpLogger{},
				"foo",
				tc.mocks.stream,
				domain.NoOpMessageHandler{},
			)
			ps := NewPollingStatusMetric(prometheus.NewRegistry())

			SaasStreamOnConnect(log.NoOpLogger{}, tc.mocks.health, tc.mocks.reloadConfig, redisStream, ps)()

			t.Log("Then the stream status will become healthy")
			assert.Equal(t, tc.expected.streamHealth, tc.mocks.health.getHealth())

			t.Log("And a connect event will be sent down the redis stream")
			assert.Equal(t, tc.expected.events, tc.mocks.stream.events)
		})
	}
}
