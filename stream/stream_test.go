package stream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/harness-community/sse/v3"
	"github.com/stretchr/testify/assert"
	"gopkg.in/cenkalti/backoff.v1"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

type mockSubscriber struct {
	sub func() (interface{}, error)
}

func (m *mockSubscriber) Close(channel string) error {
	return nil
}

func (m *mockSubscriber) Sub(ctx context.Context, channel string, id string, fn domain.HandleMessageFn) error {
	v, err := m.sub()
	if err != nil {
		return err
	}

	return fn("", v)
}

func (m *mockSubscriber) Pub(ctx context.Context, channel string, msg interface{}) error { return nil }

// resumingSubscriber delivers one message per Sub call with an incrementing ID, then disconnects,
// and records the ID that each Sub call was asked to start from
type resumingSubscriber struct {
	calls   int
	fromIDs chan string
}

func (r *resumingSubscriber) Close(channel string) error { return nil }

func (r *resumingSubscriber) Sub(ctx context.Context, channel string, id string, fn domain.HandleMessageFn) error {
	r.calls++
	select {
	case r.fromIDs <- id:
	case <-ctx.Done():
		return ctx.Err()
	}

	if err := fn(fmt.Sprintf("%d-0", r.calls), &sse.Event{Data: []byte(`{}`)}); err != nil {
		return err
	}
	return io.EOF
}

func (r *resumingSubscriber) Pub(ctx context.Context, channel string, msg interface{}) error {
	return nil
}

type mockMsgHandler struct {
	msg chan struct{}
}

func (m *mockMsgHandler) HandleMessage(ctx context.Context, msg domain.SSEMessage) error {
	m.msg <- struct{}{}
	return nil
}

type disconnectHandler struct {
	called chan struct{}
}

func (d *disconnectHandler) foo() func() {
	return func() {
		d.called <- struct{}{}
	}
}

func TestStream_Subscribe(t *testing.T) {
	type mocks struct {
		subscriber     *mockSubscriber
		messageHandler *mockMsgHandler
	}

	type expected struct {
		messagesHandled   int
		onDisconnectCalls int
	}

	testCases := map[string]struct {
		mocks          mocks
		onDisconnecter *disconnectHandler
		expected       expected
	}{
		"Given I call Subscribe and the subscriber errors": {
			mocks: mocks{
				subscriber: &mockSubscriber{
					sub: func() (interface{}, error) {
						return nil, errors.New("an error")
					},
				},
				messageHandler: &mockMsgHandler{
					msg: make(chan struct{}),
				},
			},
			expected: expected{
				messagesHandled:   0,
				onDisconnectCalls: 0,
			},
		},
		"Given I call Subscribe the subscriber errors and I have an onDisconnect": {
			mocks: mocks{
				subscriber: &mockSubscriber{
					sub: func() (interface{}, error) {
						return nil, errors.New("an error")
					},
				},
				messageHandler: &mockMsgHandler{
					msg: make(chan struct{}),
				},
			},
			onDisconnecter: &disconnectHandler{called: make(chan struct{})},
			expected: expected{
				messagesHandled:   0,
				onDisconnectCalls: 1,
			},
		},
		"Given I call Subscribe and the subscriber returns a message that isn't an *sse.Event": {
			mocks: mocks{
				subscriber: &mockSubscriber{
					sub: func() (interface{}, error) {
						return "message", nil
					},
				},
				messageHandler: &mockMsgHandler{
					msg: make(chan struct{}),
				},
			},
			expected: expected{
				messagesHandled:   0,
				onDisconnectCalls: 0,
			},
		},
		"Given I call Subscribe and the subscriber returns a message that is an *sse.Event": {
			mocks: mocks{
				subscriber: &mockSubscriber{
					sub: func() (interface{}, error) {
						return &sse.Event{Data: []byte(`{}`)}, nil
					},
				},
				messageHandler: &mockMsgHandler{
					msg: make(chan struct{}),
				},
			},
			expected: expected{
				messagesHandled:   1,
				onDisconnectCalls: 0,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			var onDisconnect func()
			if tc.onDisconnecter != nil {
				onDisconnect = tc.onDisconnecter.foo()
			}

			p := NewStream(
				log.NewNoOpLogger(),
				"foo",
				tc.mocks.subscriber,
				tc.mocks.messageHandler,
				WithOnDisconnect(onDisconnect),
				WithBackoff(backoff.NewConstantBackOff(1*time.Millisecond)),
			)
			p.Subscribe(ctx)

			msgsHandled := 0
			for msgsHandled < tc.expected.messagesHandled {
				select {
				case <-ctx.Done():
					t.Fatal("timed out waiting for subscriber to finish")
				case <-tc.mocks.messageHandler.msg:
					msgsHandled++
				}
			}

			actualDisconnectCalls := 0
			for actualDisconnectCalls < tc.expected.onDisconnectCalls {
				select {
				case <-ctx.Done():
					t.Fatal("timed out waiting for onDisconnect calls")
				case <-tc.onDisconnecter.called:
					actualDisconnectCalls++
				}
			}
			cancel()

			assert.Equal(t, tc.expected.messagesHandled, msgsHandled)

			if tc.onDisconnecter != nil {
				assert.Equal(t, tc.expected.onDisconnectCalls, actualDisconnectCalls)
			}
		})
	}
}

func TestStream_Subscribe_ResumesFromLastMessageIDAfterDisconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	subscriber := &resumingSubscriber{fromIDs: make(chan string)}
	handler := &mockMsgHandler{msg: make(chan struct{}, 10)}

	p := NewStream(
		log.NewNoOpLogger(),
		"foo",
		subscriber,
		handler,
		WithBackoff(backoff.NewConstantBackOff(1*time.Millisecond)),
	)
	p.Subscribe(ctx)

	expected := []string{"", "1-0", "2-0"}
	actual := make([]string, 0, len(expected))
	for len(actual) < len(expected) {
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for subscriber to resubscribe")
		case id := <-subscriber.fromIDs:
			actual = append(actual, id)
		}
	}
	cancel()

	assert.Equal(t, expected, actual)
}
