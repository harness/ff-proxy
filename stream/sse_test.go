package stream

import (
	"context"
	"errors"
	"testing"

	"github.com/r3labs/sse/v2"
	"github.com/stretchr/testify/assert"
)

type mockSSEClient struct {
	sub func() error
}

func (m mockSSEClient) SubscribeWithContext(ctx context.Context, stream string, fn func(msg *sse.Event)) error {
	return m.sub()
}

func TestSSEClient_Sub(t *testing.T) {
	type mocks struct {
		sseClient mockSSEClient
	}

	type expected struct {
		err error
	}

	testCases := map[string]struct {
		mocks     mocks
		shouldErr bool
		expected  expected
	}{
		"Given the underlying SSEClient errors when I call Sub": {
			mocks: mocks{sseClient: mockSSEClient{sub: func() error {
				return errors.New("an error")
			}}},

			shouldErr: true,
			expected:  expected{err: ErrSubscribing},
		},
		"Given the underlying SSEClient doesn't error when I call Sub": {
			mocks: mocks{sseClient: mockSSEClient{sub: func() error {
				return nil
			}}},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			sc := SSEClient{sse: tc.mocks.sseClient}
			err := sc.Sub(context.Background(), "", "", nil)
			if tc.shouldErr {
				assert.NotNil(t, err)
				assert.True(t, errors.Is(err, tc.expected.err))
			} else {
				assert.Nil(t, err)
			}

		})
	}
}
