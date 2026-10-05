package stream

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

type callOrder struct {
	mu    sync.Mutex
	calls []string
}

func (c *callOrder) record(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, name)
}

func (c *callOrder) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.calls))
	copy(out, c.calls)
	return out
}

type mockHealth struct {
	*sync.Mutex
	healthy   bool
	status    domain.StreamStatus
	statusErr error
	order     *callOrder
}

func (m *mockHealth) SetUnhealthy(ctx context.Context) error {
	m.Lock()
	defer m.Unlock()
	m.healthy = false

	return nil
}

func (m *mockHealth) SetHealthy(ctx context.Context) error {
	if m.order != nil {
		m.order.record("SetHealthy")
	}
	m.Lock()
	defer m.Unlock()
	m.healthy = true

	return nil
}

func (m *mockHealth) Status(ctx context.Context) (domain.StreamStatus, error) {
	return m.status, m.statusErr
}

func (m *mockHealth) getHealth() bool {
	m.Lock()
	defer m.Unlock()
	return m.healthy
}

type mockStream struct {
	events []interface{}
	order  *callOrder
	pubErr error
}

func (m *mockStream) Pub(ctx context.Context, channel string, msg interface{}) error {
	if m.order != nil {
		m.order.record("Pub")
	}
	if m.pubErr != nil {
		return m.pubErr
	}
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
					domain.SSEMessage{Event: "stream_action", Domain: domain.StreamStateDisconnected.String(), Identifier: "", Version: 0, Environment: "", Environments: []string(nil), APIKey: ""},
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
					domain.SSEMessage{Event: "stream_action", Domain: domain.StreamStateConnected.String(), Identifier: "", Version: 0, Environment: "", Environments: []string(nil), APIKey: ""},
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

// TestSaasStreamOnConnect_ReloadAfterHealthy locks FFM-13187 Bug 1:
// on reconnect, /stream gates (SetHealthy + replica CONNECTED) must open
// before reloadConfig() publishes catch-up diffs into Pushpin.
func TestSaasStreamOnConnect_ReloadAfterHealthy(t *testing.T) {
	connectedEvent := domain.SSEMessage{Event: "stream_action", Domain: domain.StreamStateConnected.String()}

	t.Run("reconnect from DISCONNECTED opens gates before reloadConfig", func(t *testing.T) {
		order := &callOrder{}
		health := &mockHealth{
			Mutex:   &sync.Mutex{},
			healthy: false,
			status:  domain.StreamStatus{State: domain.StreamStateDisconnected},
			order:   order,
		}
		ms := &mockStream{events: []interface{}{}, order: order}
		reloadCalled := 0

		redisStream := NewStream(log.NoOpLogger{}, "foo", ms, domain.NoOpMessageHandler{})
		ps := NewPollingStatusMetric(prometheus.NewRegistry())

		SaasStreamOnConnect(log.NoOpLogger{}, health, func() error {
			order.record("reloadConfig")
			reloadCalled++
			return nil
		}, redisStream, ps)()

		assert.Equal(t, []string{"SetHealthy", "Pub", "reloadConfig"}, order.snapshot())
		assert.Equal(t, 1, reloadCalled)
		assert.True(t, health.getHealth())
		assert.Equal(t, []interface{}{connectedEvent}, ms.events)
	})

	t.Run("startup CONNECTED does not call reloadConfig", func(t *testing.T) {
		order := &callOrder{}
		health := &mockHealth{
			Mutex:   &sync.Mutex{},
			healthy: false,
			status:  domain.StreamStatus{State: domain.StreamStateConnected},
			order:   order,
		}
		ms := &mockStream{events: []interface{}{}, order: order}
		reloadCalled := 0

		redisStream := NewStream(log.NoOpLogger{}, "foo", ms, domain.NoOpMessageHandler{})
		ps := NewPollingStatusMetric(prometheus.NewRegistry())

		SaasStreamOnConnect(log.NoOpLogger{}, health, func() error {
			order.record("reloadConfig")
			reloadCalled++
			return nil
		}, redisStream, ps)()

		assert.Equal(t, []string{"SetHealthy", "Pub"}, order.snapshot())
		assert.Equal(t, 0, reloadCalled)
		assert.True(t, health.getHealth())
		assert.Equal(t, []interface{}{connectedEvent}, ms.events)
	})

	t.Run("replica Pub failure still reloads config after SetHealthy", func(t *testing.T) {
		order := &callOrder{}
		health := &mockHealth{
			Mutex:   &sync.Mutex{},
			healthy: false,
			status:  domain.StreamStatus{State: domain.StreamStateDisconnected},
			order:   order,
		}
		ms := &mockStream{events: []interface{}{}, order: order, pubErr: errors.New("redis down")}
		reloadCalled := 0

		redisStream := NewStream(log.NoOpLogger{}, "foo", ms, domain.NoOpMessageHandler{})
		ps := NewPollingStatusMetric(prometheus.NewRegistry())

		SaasStreamOnConnect(log.NoOpLogger{}, health, func() error {
			order.record("reloadConfig")
			reloadCalled++
			return nil
		}, redisStream, ps)()

		assert.Equal(t, []string{"SetHealthy", "Pub", "reloadConfig"}, order.snapshot())
		assert.Equal(t, 1, reloadCalled)
		assert.True(t, health.getHealth())
		assert.Empty(t, ms.events)
	})
}
