package metricsservice

import (
	"context"
	"errors"
	"testing"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/stretchr/testify/assert"
)

type mockStream struct {
	pub func() error
}

func (m mockStream) Pub(ctx context.Context, channel string, value interface{}) error {
	return m.pub()
}

func TestMetricsStream_StoreMetrics(t *testing.T) {
	testCases := map[string]struct {
		mockStream mockStream
		shouldErr  bool
	}{
		"Given the I call StoreMetrics and the stream errors": {
			mockStream: mockStream{pub: func() error {
				return errors.New("an error")
			}},
			shouldErr: true,
		},
		"Given the I call StoreMetrics and the stream doesn't error": {
			mockStream: mockStream{pub: func() error {
				return nil
			}},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			ms := NewStream(tc.mockStream)
			err := ms.StoreMetrics(context.Background(), domain.MetricsRequest{})
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
