package cache

import (
	"container/heap"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/harness/ff-proxy/v2/log"
)

type retryItem struct {
	domain     string // "flag" or "target-segment"
	env        string
	identifier string
	attempts   int
	retryAfter time.Time
	retryFn    func(ctx context.Context) error
	index      int // index in the heap, maintained by heap.Interface
}

// retryHeap implements heap.Interface, ordered by retryAfter (earliest first).
type retryHeap []*retryItem

func (h retryHeap) Len() int            { return len(h) }
func (h retryHeap) Less(i, j int) bool   { return h[i].retryAfter.Before(h[j].retryAfter) }
func (h retryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *retryHeap) Push(x any) {
	item := x.(*retryItem)
	item.index = len(*h)
	*h = append(*h, item)
}

func (h *retryHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	item.index = -1
	*h = old[:n-1]
	return item
}

// RetryQueue is an in-memory priority queue that retries failed SSE event
// handler calls with exponential backoff. It is used to handle transient
// errors caused by database replica lag (400/404 from SaaS API).
type RetryQueue struct {
	log          log.Logger
	mu           sync.Mutex
	heap         retryHeap
	inQueue      map[string]*retryItem // dedup map keyed by "domain:env:identifier"
	maxRetries   int
	initialDelay time.Duration
	maxDelay     time.Duration
	ticker       time.Duration
	cancelFn     context.CancelFunc
}

// RetryQueueOption is a functional option for configuring RetryQueue.
type RetryQueueOption func(*RetryQueue)

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(n int) RetryQueueOption {
	return func(q *RetryQueue) { q.maxRetries = n }
}

// WithInitialDelay sets the initial retry delay.
func WithInitialDelay(d time.Duration) RetryQueueOption {
	return func(q *RetryQueue) { q.initialDelay = d }
}

// WithMaxDelay sets the maximum delay between retries.
func WithMaxDelay(d time.Duration) RetryQueueOption {
	return func(q *RetryQueue) { q.maxDelay = d }
}

// WithTickerInterval sets the ticker interval for processing the queue.
func WithTickerInterval(d time.Duration) RetryQueueOption {
	return func(q *RetryQueue) { q.ticker = d }
}

// NewRetryQueue creates and starts a RetryQueue. The background goroutine
// stops when the provided context is cancelled or Close() is called.
func NewRetryQueue(l log.Logger, ctx context.Context, opts ...RetryQueueOption) *RetryQueue {
	ctx, cancel := context.WithCancel(ctx)

	q := &RetryQueue{
		log:          l.With("component", "RetryQueue"),
		heap:         make(retryHeap, 0),
		inQueue:      make(map[string]*retryItem),
		maxRetries:   10,
		initialDelay: 5 * time.Second,
		maxDelay:     80 * time.Second,
		ticker:       1 * time.Second,
		cancelFn:     cancel,
	}

	for _, opt := range opts {
		opt(q)
	}

	heap.Init(&q.heap)
	go q.run(ctx)

	return q
}

func (q *RetryQueue) run(ctx context.Context) {
	t := time.NewTicker(q.ticker)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			q.processNext(ctx)
		}
	}
}

func (q *RetryQueue) processNext(ctx context.Context) {
	item := q.popReady()
	if item == nil {
		return
	}

	key := retryKey(item.domain, item.env, item.identifier)

	err := item.retryFn(ctx)
	if err == nil {
		q.log.Info("retry succeeded", "domain", item.domain, "env", item.env, "identifier", item.identifier, "attempt", item.attempts+1)
		q.mu.Lock()
		defer q.mu.Unlock()
		delete(q.inQueue, key)
		return
	}

	item.attempts++

	q.mu.Lock()
	defer q.mu.Unlock()

	if item.attempts >= q.maxRetries {
		q.log.Error("retry exhausted, dropping item", "domain", item.domain, "env", item.env, "identifier", item.identifier, "attempts", item.attempts, "err", err)
		delete(q.inQueue, key)
		return
	}

	// Re-enqueue with exponential backoff: initialDelay * 2^attempts, capped at maxDelay
	delay := min(q.initialDelay*(1<<uint(item.attempts)), q.maxDelay)
	item.retryAfter = time.Now().Add(delay)

	q.log.Warn("retry failed, re-enqueuing with backoff", "domain", item.domain, "env", item.env, "identifier", item.identifier, "attempt", item.attempts, "next_retry_in", delay, "err", err)

	heap.Push(&q.heap, item)
}

// popReady returns the next item whose retryAfter has passed, or nil if none is ready.
func (q *RetryQueue) popReady() *retryItem {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.heap.Len() == 0 {
		return nil
	}

	if time.Now().Before(q.heap[0].retryAfter) {
		return nil
	}

	return heap.Pop(&q.heap).(*retryItem)
}

// Enqueue adds an item to the retry queue. If an item with the same
// (domain, env, identifier) is already queued, it is skipped (dedup).
func (q *RetryQueue) Enqueue(domain, env, identifier string, retryFn func(ctx context.Context) error) {
	key := retryKey(domain, env, identifier)

	q.mu.Lock()
	defer q.mu.Unlock()

	if _, exists := q.inQueue[key]; exists {
		q.log.Debug("item already in retry queue, skipping", "domain", domain, "env", env, "identifier", identifier)
		return
	}

	item := &retryItem{
		domain:     domain,
		env:        env,
		identifier: identifier,
		attempts:   0,
		retryAfter: time.Now().Add(q.initialDelay),
		retryFn:    retryFn,
	}

	heap.Push(&q.heap, item)
	q.inQueue[key] = item
}

// Close stops the background processing goroutine.
func (q *RetryQueue) Close() {
	q.cancelFn()
}

func retryKey(domain, env, identifier string) string {
	return fmt.Sprintf("%s:%s:%s", domain, env, identifier)
}
