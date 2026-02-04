package stream

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/cenkalti/backoff.v1"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

// mockReconnectStream simulates an SSE stream that can disconnect and reconnect
type mockReconnectStream struct {
	mu              sync.Mutex
	subCallCount    int
	disconnectAfter []int // disconnect after N messages for each Sub call
	currentCall     int
	messagesSent    int
}

func (m *mockReconnectStream) Sub(ctx context.Context, channel string, id string, fn domain.HandleMessageFn) error {
	m.mu.Lock()
	callIndex := m.currentCall
	m.currentCall++
	disconnectAfter := 0
	if callIndex < len(m.disconnectAfter) {
		disconnectAfter = m.disconnectAfter[callIndex]
	}
	m.subCallCount++
	m.mu.Unlock()

	// Simulate sending some messages then disconnecting
	for i := 0; i < disconnectAfter; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			m.mu.Lock()
			m.messagesSent++
			m.mu.Unlock()
			if err := fn("", "{}"); err != nil {
				return err
			}
			time.Sleep(1 * time.Millisecond)
		}
	}

	// Simulate disconnect (return from Sub)
	return errors.New("simulated disconnect")
}

func (m *mockReconnectStream) Pub(ctx context.Context, channel string, msg interface{}) error {
	return nil
}

func (m *mockReconnectStream) Close(channel string) error {
	return nil
}

func (m *mockReconnectStream) getSubCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.subCallCount
}

// handlerTracker tracks handler invocations
type handlerTracker struct {
	mu               sync.Mutex
	connectCalls     int
	disconnectCalls  int
	connectTimes     []time.Time
	disconnectTimes  []time.Time
}

func (h *handlerTracker) onConnect() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connectCalls++
	h.connectTimes = append(h.connectTimes, time.Now())
}

func (h *handlerTracker) onDisconnect() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.disconnectCalls++
	h.disconnectTimes = append(h.disconnectTimes, time.Now())
}

func (h *handlerTracker) getConnectCalls() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.connectCalls
}

func (h *handlerTracker) getDisconnectCalls() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.disconnectCalls
}

// TestStream_ReconnectHandlersAreCalled verifies that OnConnect and OnDisconnect
// handlers are called on each subscription attempt, including reconnections.
// This is a regression test for the bug where handlers were only wired to the
// SSE client level and not called reliably on reconnection attempts.
func TestStream_ReconnectHandlersAreCalled(t *testing.T) {
	testCases := map[string]struct {
		disconnectSequence     []int // number of messages before disconnect for each Sub call
		expectedConnectCalls   int
		expectedDisconnectCalls int
		description            string
	}{
		"single connect and disconnect": {
			disconnectSequence:     []int{5},
			expectedConnectCalls:   1,
			expectedDisconnectCalls: 1,
			description:            "Basic case: connect, receive messages, disconnect",
		},
		"connect -> disconnect -> reconnect -> disconnect": {
			disconnectSequence:     []int{3, 3},
			expectedConnectCalls:   2,
			expectedDisconnectCalls: 2,
			description:            "Reconnection case: handlers should be called on EACH subscription attempt",
		},
		"multiple reconnections": {
			disconnectSequence:     []int{2, 2, 2, 2},
			expectedConnectCalls:   4,
			expectedDisconnectCalls: 4,
			description:            "Multiple reconnections: handlers called 4 times each",
		},
		"immediate disconnect with reconnect": {
			disconnectSequence:     []int{0, 0, 5},
			expectedConnectCalls:   3,
			expectedDisconnectCalls: 3,
			description:            "Immediate failures followed by successful connection",
		},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			mockStream := &mockReconnectStream{
				disconnectAfter: tc.disconnectSequence,
			}

			tracker := &handlerTracker{}

			msgHandler := &mockMsgHandler{msg: make(chan struct{}, 100)}

			s := NewStream(
				log.NewNoOpLogger(),
				"test-topic",
				mockStream,
				msgHandler,
				WithOnConnect(tracker.onConnect),
				WithOnDisconnect(tracker.onDisconnect),
				WithBackoff(backoff.NewConstantBackOff(1*time.Millisecond)),
			)

			s.Subscribe(ctx)

			// Wait for all subscription attempts to complete
			expectedSubCalls := len(tc.disconnectSequence)
			require.Eventually(t, func() bool {
				return mockStream.getSubCallCount() >= expectedSubCalls
			}, 3*time.Second, 10*time.Millisecond,
				"expected %d Sub calls, got %d", expectedSubCalls, mockStream.getSubCallCount())

			// Give a little time for final handlers to be called
			time.Sleep(50 * time.Millisecond)
			cancel()

			// Verify handler call counts
			assert.GreaterOrEqual(t, tracker.getConnectCalls(), tc.expectedConnectCalls,
				"%s: OnConnect should be called at least %d times", tc.description, tc.expectedConnectCalls)
			assert.GreaterOrEqual(t, tracker.getDisconnectCalls(), tc.expectedDisconnectCalls,
				"%s: OnDisconnect should be called at least %d times", tc.description, tc.expectedDisconnectCalls)
		})
	}
}

// timedMockStream tracks when Sub starts and ends
type timedMockStream struct {
	mu              sync.Mutex
	disconnectAfter int
	subStartTime    time.Time
	subEndTime      time.Time
	subEnded        atomic.Bool
}

func (m *timedMockStream) Sub(ctx context.Context, channel string, id string, fn domain.HandleMessageFn) error {
	m.mu.Lock()
	m.subStartTime = time.Now()
	disconnectAfter := m.disconnectAfter
	m.mu.Unlock()

	// Simulate sending messages then disconnecting
	for i := 0; i < disconnectAfter; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := fn("", "{}"); err != nil {
				return err
			}
			time.Sleep(1 * time.Millisecond)
		}
	}

	m.mu.Lock()
	m.subEndTime = time.Now()
	m.mu.Unlock()
	m.subEnded.Store(true)

	return errors.New("simulated disconnect")
}

func (m *timedMockStream) Pub(ctx context.Context, channel string, msg interface{}) error {
	return nil
}

func (m *timedMockStream) Close(channel string) error {
	return nil
}

// TestStream_BothHandlersAreCalled verifies that both OnConnect and OnDisconnect
// are called during the subscription lifecycle.
func TestStream_BothHandlersAreCalled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	mockStream := &timedMockStream{
		disconnectAfter: 1, // disconnect after 1 message
	}

	tracker := &handlerTracker{}
	msgHandler := &mockMsgHandler{msg: make(chan struct{}, 10)}

	s := NewStream(
		log.NewNoOpLogger(),
		"test-topic",
		mockStream,
		msgHandler,
		WithOnConnect(tracker.onConnect),
		WithOnDisconnect(tracker.onDisconnect),
		WithBackoff(backoff.NewConstantBackOff(1*time.Millisecond)),
	)

	s.Subscribe(ctx)

	// Wait for first subscription attempt to complete
	require.Eventually(t, func() bool {
		return mockStream.subEnded.Load()
	}, 2*time.Second, 10*time.Millisecond)

	// Give time for disconnect handler
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Verify both handlers were called
	assert.GreaterOrEqual(t, tracker.getConnectCalls(), 1, "OnConnect should be called at least once")
	assert.GreaterOrEqual(t, tracker.getDisconnectCalls(), 1, "OnDisconnect should be called at least once")
}

// TestStream_HandlersCalledOnImmediateFailure verifies that handlers are called
// even when the connection fails immediately (Sub returns error right away).
func TestStream_HandlersCalledOnImmediateFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	failCount := 0
	mockStream := &mockReconnectStream{
		disconnectAfter: []int{0, 0, 0}, // immediate failures
	}

	tracker := &handlerTracker{}
	msgHandler := &mockMsgHandler{msg: make(chan struct{}, 10)}

	s := NewStream(
		log.NewNoOpLogger(),
		"test-topic",
		mockStream,
		msgHandler,
		WithOnConnect(func() {
			tracker.onConnect()
			failCount++
		}),
		WithOnDisconnect(tracker.onDisconnect),
		WithBackoff(backoff.NewConstantBackOff(1*time.Millisecond)),
	)

	s.Subscribe(ctx)

	// Wait for multiple subscription attempts
	require.Eventually(t, func() bool {
		return mockStream.getSubCallCount() >= 3
	}, 2*time.Second, 10*time.Millisecond)

	time.Sleep(50 * time.Millisecond)
	cancel()

	// Even with immediate failures, both handlers should be called each time
	assert.GreaterOrEqual(t, tracker.getConnectCalls(), 3,
		"OnConnect should be called at least 3 times even with immediate failures")
	assert.GreaterOrEqual(t, tracker.getDisconnectCalls(), 3,
		"OnDisconnect should be called at least 3 times even with immediate failures")
}

// TestStream_NilHandlersDoNotPanic verifies that nil handlers don't cause panics.
func TestStream_NilHandlersDoNotPanic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	mockStream := &mockReconnectStream{
		disconnectAfter: []int{1, 1},
	}
	msgHandler := &mockMsgHandler{msg: make(chan struct{}, 10)}

	// Create stream WITHOUT handlers (the bug scenario before the fix)
	s := NewStream(
		log.NewNoOpLogger(),
		"test-topic",
		mockStream,
		msgHandler,
		WithBackoff(backoff.NewConstantBackOff(1*time.Millisecond)),
	)

	// This should not panic even with nil handlers
	assert.NotPanics(t, func() {
		s.Subscribe(ctx)
		time.Sleep(100 * time.Millisecond)
		cancel()
	})
}

