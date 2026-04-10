package cache

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/harness/ff-proxy/v2/log"
	"github.com/stretchr/testify/assert"
)

func TestRetryQueue_EnqueueAndProcess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var called int32
	q := NewRetryQueue(log.NewNoOpLogger(), ctx,
		WithInitialDelay(10*time.Millisecond),
		WithTickerInterval(5*time.Millisecond),
	)
	defer q.Close()

	q.Enqueue("flag", "env1", "feat1", func(ctx context.Context) error {
		atomic.AddInt32(&called, 1)
		return nil
	})

	assert.Eventually(t, func() bool {
		return atomic.LoadInt32(&called) == 1
	}, 500*time.Millisecond, 5*time.Millisecond, "retry function should be called once")

	// Item should be removed from inQueue after success
	q.mu.Lock()
	_, exists := q.inQueue["flag:env1:feat1"]
	q.mu.Unlock()
	assert.False(t, exists, "item should be removed from inQueue after success")
}

func TestRetryQueue_Dedup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var called int32
	q := NewRetryQueue(log.NewNoOpLogger(), ctx,
		WithInitialDelay(1*time.Hour), // very long delay so nothing fires
		WithTickerInterval(5*time.Millisecond),
	)
	defer q.Close()

	fn := func(ctx context.Context) error {
		atomic.AddInt32(&called, 1)
		return nil
	}

	q.Enqueue("flag", "env1", "feat1", fn)
	q.Enqueue("flag", "env1", "feat1", fn) // duplicate

	q.mu.Lock()
	heapLen := q.heap.Len()
	q.mu.Unlock()

	assert.Equal(t, 1, heapLen, "duplicate should not be added to heap")
}

func TestRetryQueue_ExponentialBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var attempts int32
	q := NewRetryQueue(log.NewNoOpLogger(), ctx,
		WithInitialDelay(10*time.Millisecond),
		WithTickerInterval(5*time.Millisecond),
		WithMaxRetries(3),
	)
	defer q.Close()

	q.Enqueue("flag", "env1", "feat1", func(ctx context.Context) error {
		atomic.AddInt32(&attempts, 1)
		return errors.New("still failing")
	})

	// Should exhaust all 3 retries
	assert.Eventually(t, func() bool {
		return atomic.LoadInt32(&attempts) >= 3
	}, 2*time.Second, 5*time.Millisecond, "should attempt maxRetries times")

	// After exhaustion, item should be removed from inQueue
	assert.Eventually(t, func() bool {
		q.mu.Lock()
		defer q.mu.Unlock()
		_, exists := q.inQueue["flag:env1:feat1"]
		return !exists
	}, 500*time.Millisecond, 5*time.Millisecond, "item should be removed after max retries exhausted")
}

func TestRetryQueue_SuccessStopsRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var attempts int32
	q := NewRetryQueue(log.NewNoOpLogger(), ctx,
		WithInitialDelay(10*time.Millisecond),
		WithTickerInterval(5*time.Millisecond),
		WithMaxRetries(3),
	)
	defer q.Close()

	// Fail once, then succeed
	q.Enqueue("flag", "env1", "feat1", func(ctx context.Context) error {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			return errors.New("transient failure")
		}
		return nil
	})

	// Wait for success on second attempt
	assert.Eventually(t, func() bool {
		return atomic.LoadInt32(&attempts) >= 2
	}, 2*time.Second, 5*time.Millisecond)

	// Give a bit of time to ensure no more retries happen
	time.Sleep(100 * time.Millisecond)

	finalAttempts := atomic.LoadInt32(&attempts)
	assert.Equal(t, int32(2), finalAttempts, "should stop retrying after success")

	q.mu.Lock()
	_, exists := q.inQueue["flag:env1:feat1"]
	q.mu.Unlock()
	assert.False(t, exists, "item should be removed from inQueue after success")
}

func TestRetryQueue_Close(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var called int32
	q := NewRetryQueue(log.NewNoOpLogger(), ctx,
		WithInitialDelay(50*time.Millisecond),
		WithTickerInterval(5*time.Millisecond),
	)

	q.Enqueue("flag", "env1", "feat1", func(ctx context.Context) error {
		atomic.AddInt32(&called, 1)
		return nil
	})

	// Close immediately before the initial delay expires
	q.Close()

	time.Sleep(150 * time.Millisecond)

	assert.Equal(t, int32(0), atomic.LoadInt32(&called), "retry function should not be called after Close")
}

func TestRetryQueue_DifferentDomainsSameIdentifier(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q := NewRetryQueue(log.NewNoOpLogger(), ctx,
		WithInitialDelay(1*time.Hour),
		WithTickerInterval(5*time.Millisecond),
	)
	defer q.Close()

	fn := func(ctx context.Context) error { return nil }

	q.Enqueue("flag", "env1", "id1", fn)
	q.Enqueue("target-segment", "env1", "id1", fn)

	q.mu.Lock()
	heapLen := q.heap.Len()
	q.mu.Unlock()

	assert.Equal(t, 2, heapLen, "different domains with same identifier should both be enqueued")
}
