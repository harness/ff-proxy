package metricsservice

import (
	"context"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

const (
	maxEvaluationQueueSize = 1 << 20 // 1MB
	maxTargetQueueSize     = 1 << 20
)

// Queue is an in memory queue storing metrics requests. It flushes its contents
// whenever the max queue size in MB has been reached or the ticker expires, whichever
// occurs first. It exposes a channel via the Listen() method where it writes metrics
// to before flushing. This allows other processes to receive the metrics once the queue
// is full.
type Queue struct {
	log log.Logger
	//that is used to flush...
	queue       chan map[string]domain.MetricsRequest
	metricsData *metricsMap
	targetData  *metricsMap

	metricsTicker *time.Ticker
	targetsTicker *time.Ticker

	metricsDuration time.Duration
	targetsDuration time.Duration
}

// NewQueue creates a Queue
func NewQueue(ctx context.Context, l log.Logger, duration time.Duration) Queue {
	l.With("component", "Queue")
	q := Queue{
		log:             l,
		queue:           make(chan map[string]domain.MetricsRequest),
		metricsDuration: duration,
		targetsDuration: duration,
		metricsTicker:   time.NewTicker(duration),
		targetsTicker:   time.NewTicker(duration),
		metricsData:     newMetricsMap(),
		targetData:      newMetricsMap(),
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

		case <-q.metricsTicker.C:
			metrics := q.metricsData.get()
			if len(metrics) == 0 {
				continue
			}
			if err := send(ctx, q.queue, metrics); err != nil {
				// The only possible error here is a context canceled or deadline exceeded
				// but lets still log it anyway
				q.log.Error("unable to flush metrics to channel", "method", "flush", "err", err)
			}
			q.metricsData.flush()
		case <-q.targetsTicker.C:
			metrics := q.targetData.get()
			if len(metrics) == 0 {
				continue
			}
			if err := send(ctx, q.queue, metrics); err != nil {
				// The only possible error here is a context canceled or deadline exceeded
				// but lets still log it anyway
				q.log.Error("unable to flush metrics to channel", "method", "flush", "err", err)
			}
			q.targetData.flush()
		}
	}
}

// StoreMetrics adds a metrics request to the queue
func (q Queue) StoreMetrics(ctx context.Context, m domain.MetricsRequest) error {

	// take a copy of the requests and delete variant
	tRequest := m
	mRequest := m
	tRequest.MetricsData = nil
	mRequest.TargetData = nil

	// handle client metrics
	err := q.handleMetricsData(ctx, mRequest)
	if err != nil {
		return err
	}
	// handle target metrics
	err = q.handleTargetData(ctx, tRequest)
	if err != nil {
		return err
	}
	return nil
}

func (q Queue) handleMetricsData(ctx context.Context, m domain.MetricsRequest) error {
	// If we've no metric data to handle then we can exit early
	if m.MetricsData == nil {
		return nil
	}

	// we are aggregating the metrics Data and set it to its map.
	if q.metricsData.size() < maxEvaluationQueueSize {
		aggregatedMetricsData, err := q.metricsData.aggregate(m)
		if err != nil {
			q.log.Error("unable to aggregate metrics data", "method", "StoreMetrics", "err", err)
			return err
		}
		originalSize := len(*m.MetricsData)
		aggregatedSize := len(aggregatedMetricsData)
		// set aggregated data to be stored
		m.MetricsData = &aggregatedMetricsData
		// aggregate the list.
		q.log.Debug("aggregated metrics data", "originalSize", originalSize, "aggregatedSize", aggregatedSize)
		q.metricsData.add(m)
		return nil
	}
	metrics := q.metricsData.get()
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
	q.metricsTicker.Reset(q.metricsDuration)
	q.metricsData.flush()
	q.metricsData.add(m)
	return nil
}
func (q Queue) handleTargetData(ctx context.Context, m domain.MetricsRequest) error {
	// If we've no target data to handle then we can exit early
	if m.TargetData == nil {
		return nil
	}

	// check if we have maxed out target metrics
	if q.targetData.size() < maxTargetQueueSize {
		//add and  increment for target
		q.targetData.add(m)
		return nil
	}

	metrics := q.targetData.get()
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
	q.targetsTicker.Reset(q.targetsDuration)
	q.targetData.flush()
	q.targetData.add(m)
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
