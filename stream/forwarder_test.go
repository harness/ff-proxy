package stream

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/harness/ff-proxy/v2/domain"

	"github.com/stretchr/testify/assert"

	"github.com/harness/ff-proxy/v2/log"
)

type mockPublisher struct {
	*sync.Mutex
	pub             func() error
	eventsForwarded int
}

func (m *mockPublisher) Pub(ctx context.Context, channel string, value interface{}) error {
	m.Lock()
	defer m.Unlock()
	m.eventsForwarded++
	return m.pub()
}

func (m *mockPublisher) getEventsForwarded() int {
	m.Lock()
	defer m.Unlock()
	return m.eventsForwarded
}

type mockMessageHandler struct {
	handleMessage func() error
}

func (m mockMessageHandler) HandleMessage(ctx context.Context, msg domain.SSEMessage) error {
	return m.handleMessage()
}

func TestForwarder_HandleMesssage(t *testing.T) {
	type args struct {
		message domain.SSEMessage
	}

	type mocks struct {
		publisher      *mockPublisher
		messageHandler mockMessageHandler
	}

	type expected struct {
		eventsForwarded int
	}

	testCases := map[string]struct {
		args      args
		mocks     mocks
		expected  expected
		shouldErr bool
	}{
		"Given the wrapper MessageHandler fails to handle the message": {
			args: args{message: domain.SSEMessage{}},
			mocks: mocks{
				publisher: &mockPublisher{Mutex: &sync.Mutex{}},
				messageHandler: mockMessageHandler{handleMessage: func() error {
					return errors.New("an error")
				}},
			},
			expected: expected{
				eventsForwarded: 0,
			},
			shouldErr: true,
		},
		"Given I have an SSEMessage with an empty domain": {
			args: args{
				message: domain.SSEMessage{
					Domain: "",
				},
			},
			mocks: mocks{
				publisher: &mockPublisher{Mutex: &sync.Mutex{}},
				messageHandler: mockMessageHandler{handleMessage: func() error {
					return nil
				}},
			},
			expected: expected{
				eventsForwarded: 0,
			},
			shouldErr: false,
		},
		"Given I have an SSEMessage with a domain that isn't 'flag' or 'target-segment'": {
			args: args{
				message: domain.SSEMessage{
					Domain: "foo",
				},
			},
			mocks: mocks{
				publisher: &mockPublisher{Mutex: &sync.Mutex{}},
				messageHandler: mockMessageHandler{handleMessage: func() error {
					return nil
				}},
			},
			expected: expected{
				eventsForwarded: 0,
			},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'flag' but the stream fails to publish": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainFeature,
				},
			},
			mocks: mocks{
				publisher: &mockPublisher{
					Mutex: &sync.Mutex{},
					pub: func() error {
						return errors.New("an error")
					},
				},
				messageHandler: mockMessageHandler{handleMessage: func() error {
					return nil
				}},
			},
			expected: expected{
				eventsForwarded: 1,
			},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'flag'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainFeature,
				},
			},
			mocks: mocks{
				publisher: &mockPublisher{
					Mutex: &sync.Mutex{},
					pub: func() error {
						return nil
					},
				},
				messageHandler: mockMessageHandler{handleMessage: func() error {
					return nil
				}},
			},
			expected: expected{
				eventsForwarded: 1,
			},
			shouldErr: false,
		},
		"Given I have an SSEMessage with the domain 'target-segment'": {
			args: args{
				message: domain.SSEMessage{
					Domain: domain.MsgDomainSegment,
				},
			},
			mocks: mocks{
				publisher: &mockPublisher{
					Mutex: &sync.Mutex{},
					pub: func() error {
						return nil
					},
				},
				messageHandler: mockMessageHandler{handleMessage: func() error {
					return nil
				}},
			},
			expected: expected{
				eventsForwarded: 1,
			},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			s := NewForwarder(log.NewNoOpLogger(), tc.mocks.publisher, tc.mocks.messageHandler)

			err := s.HandleMessage(context.Background(), tc.args.message)
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.eventsForwarded, tc.mocks.publisher.getEventsForwarded())
		})
	}
}
