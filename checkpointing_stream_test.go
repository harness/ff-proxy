package ffproxy

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/log"
	"github.com/stretchr/testify/assert"
)

type mockCheckpointer struct {
	*sync.Mutex
	checkpoints map[string]string
	err         error
	sets        int
}

func newMockCheckpointer(err error) *mockCheckpointer {
	return &mockCheckpointer{
		Mutex:       &sync.Mutex{},
		checkpoints: make(map[string]string),
		err:         err,
	}
}

func (m *mockCheckpointer) SetKV(ctx context.Context, key string, value string) error {
	m.Lock()
	defer m.Unlock()

	if m.err != nil {
		return m.err
	}

	m.checkpoints[key] = value
	m.sets++
	return nil
}

func (m *mockCheckpointer) GetKV(ctx context.Context, key string) (string, error) {
	m.Lock()
	defer m.Unlock()

	if m.err != nil {
		return "", m.err
	}

	cp, ok := m.checkpoints[key]
	if !ok {
		return "", nil
	}
	return cp, nil
}

var testEvents = []domain.StreamEvent{
	{Checkpoint: "1-0", Values: map[domain.StreamEventValue]string{}},
	{Checkpoint: "2-0", Values: map[domain.StreamEventValue]string{}},
	{Checkpoint: "3-1", Values: map[domain.StreamEventValue]string{}},
	{Checkpoint: "3-2", Values: map[domain.StreamEventValue]string{}},
}

func TestStreamCheckpointer_Sub(t *testing.T) {
	topic := "test-topic"

	testCases := map[string]struct {
		stream       mockStream
		checkpointer *mockCheckpointer
		checkpoint   string
		shouldErr    bool
		expected     []domain.StreamEvent
	}{
		"Given I call Sub with a stream that errors": {
			stream:       newMockStream(errors.New("error"), topic, testEvents...),
			checkpointer: newMockCheckpointer(nil),
			checkpoint:   "",
			shouldErr:    true,
		},
		"Given I call Sub and don't pass a checkpoint": {
			stream:       newMockStream(nil, topic, testEvents...),
			checkpointer: newMockCheckpointer(nil),
			checkpoint:   "",
			shouldErr:    false,
			expected:     testEvents,
		},
		"Given I call Sub and I pass a checkpoint": {
			stream:       newMockStream(nil, topic, testEvents...),
			checkpointer: newMockCheckpointer(nil),
			checkpoint:   "2-0",
			shouldErr:    false,
			expected:     testEvents[1:],
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cs := NewCheckpointingStream(ctx, tc.stream, tc.checkpointer, log.NewNoOpLogger())

			actual := []domain.StreamEvent{}
			err := cs.Sub(ctx, topic, tc.checkpoint, func(event domain.StreamEvent) {
				actual = append(actual, event)
			})

			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			assert.ElementsMatch(t, tc.expected, actual)
		})
	}
}

func TestCheckpointStream_SetCheckpoint(t *testing.T) {
	outOfOrderTestEvents := []domain.StreamEvent{
		{Checkpoint: "2-0", Values: map[domain.StreamEventValue]string{}},
		{Checkpoint: "3-2", Values: map[domain.StreamEventValue]string{}},
		{Checkpoint: "3-1", Values: map[domain.StreamEventValue]string{}},
		{Checkpoint: "1-0", Values: map[domain.StreamEventValue]string{}},
	}

	topic := "test-topic"

	testCases := map[string]struct {
		expectedCheckpoint string
		checkpointer       *mockCheckpointer
		events             []domain.StreamEvent
		shouldErr          bool
		sets               int
	}{
		"Given I have events that all come through in the correct order": {
			expectedCheckpoint: "3-2",
			checkpointer:       newMockCheckpointer(nil),
			events:             testEvents,
			shouldErr:          false,
			sets:               4,
		},
		"Given I have events that don't come through in the correct order": {
			expectedCheckpoint: "3-2",
			checkpointer:       newMockCheckpointer(nil),
			events:             outOfOrderTestEvents,
			shouldErr:          false,
			sets:               2,
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			cs := NewCheckpointingStream(ctx, newMockStream(nil, topic, tc.events...), tc.checkpointer, log.NewNoOpLogger())

			i := 0
			newCtx, cancel := context.WithCancel(ctx)
			err := cs.Sub(newCtx, topic, "", func(e domain.StreamEvent) {
				if i == len(tc.events) {
					cancel()
				}
				i++
			})
			if err != nil {
				t.Fatal(err)
			}

			time.Sleep(1 * time.Second)

			actualCheckpoint, err := tc.checkpointer.GetKV(ctx, fmt.Sprintf("checkpoint-%s", topic))
			if (err != nil) != tc.shouldErr {
				t.Errorf("(%s): error = %v, shouldErr = %v", desc, err, tc.shouldErr)
			}

			assert.Equal(t, tc.expectedCheckpoint, actualCheckpoint)

			tc.checkpointer.Lock()
			assert.Equal(t, tc.sets, tc.checkpointer.sets)
			tc.checkpointer.Unlock()
		})
	}

}
