package metricsservice

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	jsoniter "github.com/json-iterator/go"
)

type metricStore interface {
	StoreMetrics(ctx context.Context, r domain.MetricsRequest) error
	Listen(ctx context.Context) <-chan map[string]domain.MetricsRequest
}

// metricService defines the interface for interacting with the Harness Saas metrics service
type metricService interface {
	PostMetrics(ctx context.Context, envID string, r domain.MetricsRequest, clusterIdentifier string) error
}

// Worker is a type that is used by the Primary Proxy to consume metrics
// from ReadReplicas and forward them on to Harness Saas.
type Worker struct {
	log               log.Logger
	subscriber        domain.Subscriber
	metricsStore      metricStore
	metricsService    metricService
	readConcurrency   int
	clusterIdentifier string
}

// NewWorker creates a Worker
func NewWorker(l log.Logger, store metricStore, metricSvc metricService, sub domain.Subscriber, readConn int, clusterIdentifer string) Worker {
	return Worker{
		log:               l,
		subscriber:        sub,
		metricsStore:      store,
		metricsService:    metricSvc,
		readConcurrency:   readConn,
		clusterIdentifier: clusterIdentifer,
	}
}

// Start starts the process whereby the Worker consumes metrics from read replicas and forwards them on to Harness Saas.
func (w Worker) Start(ctx context.Context) {
	// Start a single thread that subscribes to the redis stream
	metrics := w.subscribe(ctx)

	// Start multiple threads to process events coming off the redis stream
	for i := 0; i < w.readConcurrency; i++ {
		go w.handleMetrics(ctx, metrics)
	}

	// Start a single thread that sends metrics to Saas
	go w.postMetrics(ctx)
}

// subscribe starts a single thread that subcribes to a redis stream and writes
// events coming off the stream to a channel.
func (w Worker) subscribe(ctx context.Context) <-chan []byte {
	out := make(chan []byte)

	go func() {
		defer close(out)

		id := ""
		for {
			// There's nothing we can really do here if we error in the callback parsing/handling the message
			// so we log the errors as warnings but return nil so we carry on receiving messages from the stream
			err := w.subscriber.Sub(ctx, SDKMetricsStream, id, func(latestID string, v interface{}) error {
				// We want to keep track of the id of the latest message we've received so that if
				// we disconnect we can resume from that point
				id = latestID

				s, ok := v.(string)
				if !ok {
					w.log.Warn("unexpected message format received", "stream", SDKMetricsStream, "type", reflect.TypeOf(v))
					return nil
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case out <- []byte(s):
				}

				return nil
			})
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}

				w.log.Warn("dropped subscription to redis stream, backing off for 30 seconds and trying again", "stream", SDKMetricsStream, "err", err)

				// If we do break out of subscribe it will probably be because of a connection error with redis
				// so backoff for 30 seconds before trying again
				time.Sleep(30 * time.Second)
				continue
			}
		}
	}()

	return out
}

func (w Worker) handleMetrics(ctx context.Context, metrics <-chan []byte) {
	for {
		select {
		case <-ctx.Done():
			w.log.Info("")
			return
		case b, ok := <-metrics:
			if !ok {
				w.log.Info("")
				return
			}

			mr := domain.MetricsRequest{
				Size: len(b),
			}
			if err := jsoniter.Unmarshal(b, &mr); err != nil {
				w.log.Warn("failed to unmarshal metrics message", "stream", SDKMetricsStream, "err", err)
				continue
			}

			if err := w.metricsStore.StoreMetrics(ctx, mr); err != nil {
				w.log.Warn("failed to store metrics received from read replica", "stream", SDKMetricsStream, "err", err)
				continue
			}
		}
	}
}

func (w Worker) postMetrics(ctx context.Context) {
	for metrics := range w.metricsStore.Listen(ctx) {

		for _, _ = range metrics {
			//if err := w.metricsService.PostMetrics(ctx, envID, metric, w.clusterIdentifier); err != nil {
			//	w.log.Error("sending metrics failed", "environment", envID, "cluster_identifier", w.clusterIdentifier, "error", err)
			//}
		}
	}
}
