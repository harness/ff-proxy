package metricsservice

import (
	"context"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

const (
	maxQueueSize = 1 << 20 // 1MB
)

// Queue is an in memory queue storing metrics requests. It flushes its contents
// whenever the max queue size in MB has been reached or the ticker expires, whichever
// occurs first. It exposes a channel via the Listen() method where it writes metrics
// to before flushing. This allows other processes to receive the metrics once the queue
// is full.
type Queue struct {
	log            log.Logger
	queue          chan map[string]domain.MetricsRequest
	metrics        *metricsMap
	ticker         *time.Ticker
	tickerDuration time.Duration
}

// NewQueue creates a Queue
func NewQueue(ctx context.Context, l log.Logger, duration time.Duration) Queue {
	l.With("component", "Queue")
	ticker := time.NewTicker(duration)

	q := Queue{
		log:            l,
		queue:          make(chan map[string]domain.MetricsRequest),
		metrics:        newMetricsMap(),
		ticker:         ticker,
		tickerDuration: duration,
	}

	// Start a routine that flushes the queue when the ticker expires
	go q.flush(ctx)

	return q
}

func (q Queue) flush(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			q.log.Info("exiting Queue.flush because context was cancelled")
			return

		case <-q.ticker.C:
			metrics := q.metrics.get()
			if len(metrics) == 0 {
				continue
			}

			if err := send(context.Background(), q.queue, metrics); err != nil {
				// The only possible error here is a context canceled or deadline exceeded
				// but lets still log it anyway
				q.log.Error("unable to flush metrics to channel", "method", "flush", "err", err)
			}
			q.metrics.flush()
		}
	}
}

// StoreMetrics adds a metrics request to the queue
func (q Queue) StoreMetrics(ctx context.Context, m domain.MetricsRequest) error {
	if q.metrics.size() < maxQueueSize {
		aggregatedMetricsData := q.metrics.aggregate(m)
		originalSize := len(*m.MetricsData)
		aggregatedSize := len(aggregatedMetricsData)
		// set aggregated data to be stored
		m.MetricsData = &aggregatedMetricsData
		// aggregate the list.
		q.log.Debug("aggregated metrics data", "originalSize", originalSize, "aggregatedSize", aggregatedSize)
		q.metrics.add(m)
		return nil
	}

	metrics := q.metrics.get()
	if len(metrics) == 0 {
		return nil
	}

	if err := send(ctx, q.queue, metrics); err != nil {
		// The only possible error here is a context canceled or deadline exceeded
		// lets still bubble it up to the caller
		return err
	}

	// Flush all the existing metrics because the max size has been reached,
	// reset the ticker and add the new metric to the map
	q.ticker.Reset(q.tickerDuration)
	q.metrics.flush()
	q.metrics.add(m)
	return nil
}

// Listen returns a channel that the queue flushes metrics requests to
func (q Queue) Listen(ctx context.Context) <-chan map[string]domain.MetricsRequest {
	out := make(chan map[string]domain.MetricsRequest)

	go func() {
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-q.queue:
				if !ok {
					return
				}

				if err := send(ctx, out, v); err != nil {
					// The only possible error here is a context canceled or deadline exceeded
					// but lets still log it anyway
					q.log.Error("unable to flush metrics to channel", "method", "Listen", "err", err)
				}
			}
		}
	}()

	return out
}

// send is a helper for writing to a channel while checking if the context is done. It returns whenever we can write to
// the channel or the context expires, whichever happens first.
func send(ctx context.Context, c chan<- map[string]domain.MetricsRequest, value map[string]domain.MetricsRequest) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c <- value:
	}

	return nil
}
