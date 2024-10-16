package stream

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/prometheus/client_golang/prometheus"
)

// Health defines the health interface for a Stream
type Health interface {
	SetHealthy(ctx context.Context) error
	SetUnhealthy(ctx context.Context) error
	Status(ctx context.Context) (domain.StreamStatus, error)
}

// NewHealth is a constructor that creates a Health implementation depending on if the Proxy is a Primary or Replica
func NewHealth(l log.Logger, key string, c cache.Cache, readReplica bool) Health {
	if readReplica {
		return NewReplicaHealth(key, c, l)
	}

	return NewPrimaryHealth(key, c, l)
}

// PrimaryHealth maintains the health/status of a stream in a cache
type PrimaryHealth struct {
	log log.Logger
	c   cache.Cache
	key string

	// Also keep a copy of the status in memory. This way we can
	// recover if we failed to update the remote status in the cache
	// due to a network error and avoid getting in a stuck state.
	inMemStatus *domain.SafeStreamStatus
}

// NewPrimaryHealth creates a PrimaryHealth
func NewPrimaryHealth(k string, c cache.Cache, l log.Logger) PrimaryHealth {
	l = l.With("component", "PrimaryStreamHealth")

	defaultStreamStatus := domain.StreamStatus{
		State: domain.StreamStateInitializing,
		Since: time.Now().UnixMilli(),
	}

	h := PrimaryHealth{
		log:         l,
		key:         k,
		c:           c,
		inMemStatus: domain.NewSafeStreamStatus(defaultStreamStatus),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// It's fine for us to ignore this error, if we fail to set the status
	// to initialising we'll end up setting it as Connected or Disconnected
	// in our OnConnect/OnDisconnect handlers when we attempt to stream
	h.log.Info("setting stream status in cache", "state", defaultStreamStatus.State, "since", defaultStreamStatus.Since)
	_ = h.c.Set(ctx, h.key, defaultStreamStatus)

	return h
}

// SetHealthy sets the stream status as CONNECTED in the cache.
// If the stream status is already CONNECTED it does nothing.
func (h PrimaryHealth) SetHealthy(ctx context.Context) error {
	streamStatus := domain.StreamStatus{
		State: domain.StreamStateConnected,
		Since: time.Now().UnixMilli(),
	}

	h.log.Info("SetHealthy - Updating streamStatus", "streamStatus.State", streamStatus.State, "streamStatus.Since", streamStatus.Since)

	cachedStatus := domain.StreamStatus{}

	if err := h.c.Get(ctx, h.key, &cachedStatus); err != nil {
		// Ignore NotFound errors for this key because if the key doesn't
		// exist we'll end up setting it at the end of this function
		if !errors.Is(err, domain.ErrCacheNotFound) {
			h.inMemStatus.Set(streamStatus)
			return err
		}
	}

	// If current status is healthy then don't do anything
	if cachedStatus.State == domain.StreamStateConnected {
		return nil
	}

	h.log.Info("setting stream status in cache", "state", streamStatus.State, "since", streamStatus.Since)
	h.inMemStatus.Set(streamStatus)
	return h.c.Set(ctx, h.key, streamStatus)
}

// SetUnhealthy sets the stream status as DISCONNECTED in the cache.
// If the stream status is already DISCONNECTED it does nothing.
func (h PrimaryHealth) SetUnhealthy(ctx context.Context) error {
	streamStatus := domain.StreamStatus{
		State: domain.StreamStateDisconnected,
		Since: time.Now().UnixMilli(),
	}

	cachedStatus := domain.StreamStatus{}
	if err := h.c.Get(ctx, h.key, &cachedStatus); err != nil {
		// Ignore NotFound errors for this key because if the key doesn't
		// exist we'll end up setting it at the end of this function
		if !errors.Is(err, domain.ErrCacheNotFound) {
			h.inMemStatus.Set(streamStatus)
			return err
		}
	}

	// If current status is disconnected then we don't need to do anything
	if cachedStatus.State == domain.StreamStateDisconnected {
		return nil
	}

	h.log.Info("setting stream status in cache", "state", streamStatus.State, "since", streamStatus.Since)
	h.inMemStatus.Set(streamStatus)
	return h.c.Set(ctx, h.key, streamStatus)
}

// VerifyStreamStatus checks that the stream status recorded in the cache matches the stream status recorded in memory.
// There was an issue where if we failed to update the stream status in the cache due to a network error that it could
// be stuck as INITIALIZING until there was a disconnect between the Harness Saas stream and the Proxy. During this time
// where the state was stuck as INITIALIZING any SDK stream requests would be rejected by replicas.
//
// This thread should resolve that issue because the stream status stored in memory isn't affected by network issues so
// should always be up to date.
func (h PrimaryHealth) VerifyStreamStatus(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.log.Info("context canceled, stopping thread that checks the cached stream status matches the in memory status")
			return
		case <-ticker.C:

			var cachedStatus domain.StreamStatus
			inMemStatus := h.inMemStatus.Get()

			if err := h.c.Get(ctx, h.key, &cachedStatus); err != nil {
				h.log.Error("failed to get stream status from cache", "err", err)
			}

			h.log.Info("verifying stream status", "in_mem_status_state", inMemStatus.State, "in_mem_status_since", inMemStatus.Since, "cached_status_state", cachedStatus.State, "cached_status_since", cachedStatus.Since)

			// The inMemState should always be accurate, if there's a difference between it and the
			// cachedState then it's possible there was a network error when we tried to update the
			// cachedState in SetHealthy or SetUnhealthy and we should try to update the cachedState again
			if cachedStatus.State != inMemStatus.State {
				h.log.Info("setting stream status in cache", "state", inMemStatus.State, "since", inMemStatus.Since)
				if err := h.c.Set(ctx, h.key, inMemStatus); err != nil {
					h.log.Error("failed to update cached stream state to match in memory stream state", "err", err)
				}
			}
		}
	}
}

// Status returns the StreamStatus from the cache
func (h PrimaryHealth) Status(_ context.Context) (domain.StreamStatus, error) {
	return h.inMemStatus.Get(), nil
}

type getter interface {
	Get(ctx context.Context, key string, v interface{}) error
}

// ReplicaHealth is a Health implementation that's used when the Proxy is running as a read replica
type ReplicaHealth struct {
	log log.Logger
	c   getter
	key string

	// Also keep a copy of the status in memory. This way we can
	// recover if we failed to update the remote status in the cache
	// due to a network error and avoid getting in a stuck state.
	inMemStatus *domain.SafeStreamStatus
}

// NewReplicaHealth creates a ReplicaHealth
func NewReplicaHealth(k string, c cache.Cache, l log.Logger) ReplicaHealth {
	l = l.With("component", "ReplicaStreamHealth")

	defaultStreamStatus := domain.StreamStatus{
		State: domain.StreamStateInitializing,
		Since: time.Now().UnixMilli(),
	}

	h := ReplicaHealth{
		log:         l,
		key:         k,
		c:           c,
		inMemStatus: domain.NewSafeStreamStatus(defaultStreamStatus),
	}

	return h
}

// SetHealthy sets the in memory stream status in the read replica to CONNECTED
func (r ReplicaHealth) SetHealthy(_ context.Context) error {
	currentStatus := r.inMemStatus.Get()

	// If we're already connected we don't need to modify the status
	if currentStatus.State == domain.StreamStateConnected {
		return nil
	}

	r.inMemStatus.Set(domain.StreamStatus{
		State: domain.StreamStateConnected,
		Since: time.Now().UnixMilli(),
	})

	return nil
}

// SetUnhealthy sets the in memory stream status in the read replica to DISCONNECTED
func (r ReplicaHealth) SetUnhealthy(_ context.Context) error {
	currentStatus := r.inMemStatus.Get()

	// If we're already disconnected we don't need to modify the status
	if currentStatus.State == domain.StreamStateDisconnected {
		return nil
	}

	r.inMemStatus.Set(domain.StreamStatus{
		State: domain.StreamStateDisconnected,
		Since: time.Now().UnixMilli(),
	})

	return nil
}

// Status returns the read replicas in memory stream status
func (r ReplicaHealth) Status(_ context.Context) (domain.StreamStatus, error) {
	return r.inMemStatus.Get(), nil
}

type streamHealthMetrics struct {
	next     Health
	gauge    *prometheus.GaugeVec
	hostName string
}

// NewStreamHealthMetrics creates a new Health implementation 'middleware' that tracks the state
// of streaming with a prometheus metric before calling the next Health implementation.
func NewStreamHealthMetrics(next Health, r prometheus.Registerer) Health {
	hostName, _ := os.Hostname()
	if hostName == "" {
		hostName = "unknown"
	}

	h := streamHealthMetrics{
		next:     next,
		hostName: hostName,
		gauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			// Tracks the health of Proxy streaming i.e. are we connected to Saas stream and
			// are SDKs connected to Replica streams. Or are we disconnected fromm Saas & polling
			// in which case SDKs would also be disconnected from replicas and polling
			Name: "ff_proxy_stream_health",
			Help: "Tracks the health of Proxy streaming i.e. is the Proxy connected to the Saas stream & are sdks connected to the Replica streams",
		},
			[]string{"host"},
		),
	}

	r.MustRegister(h.gauge)
	return h
}

// SetHealthy sets the gauge value to connected (1) and calls the next Health implementation
func (p streamHealthMetrics) SetHealthy(ctx context.Context) error {
	p.gauge.WithLabelValues(p.hostName).Set(1)
	return p.next.SetHealthy(ctx)
}

// SetUnhealthy sets the gauge value to disconnected (0) and calls the next Health implementation
func (p streamHealthMetrics) SetUnhealthy(ctx context.Context) error {
	p.gauge.WithLabelValues(p.hostName).Set(0)
	return p.next.SetUnhealthy(ctx)
}

func (p streamHealthMetrics) Status(ctx context.Context) (domain.StreamStatus, error) {
	return p.next.Status(ctx)
}

// PollingStatusMetric is a metric that tracks the state of polling in the Primary Proxy
type PollingStatusMetric struct {
	gauge    *prometheus.GaugeVec
	hostName string
}

// NewPollingStatusMetric creates a new PollingStatusMetric
func NewPollingStatusMetric(r prometheus.Registerer) PollingStatusMetric {
	hostName, _ := os.Hostname()
	if hostName == "" {
		hostName = "unknown"
	}

	p := PollingStatusMetric{
		hostName: hostName,
		gauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "ff_proxy_polling_status",
			Help: "Tracks whether or not the Primary Proxy is polling",
		},
			[]string{"host"},
		),
	}

	r.MustRegister(p.gauge)
	return p
}

// Polling sets the gauge to the value for when we're in polling mode
func (p PollingStatusMetric) Polling() {
	p.gauge.WithLabelValues(p.hostName).Set(1)
}

// NotPolling sets the gauge to the value for when we're not in polling mode
func (p PollingStatusMetric) NotPolling() {
	p.gauge.WithLabelValues(p.hostName).Set(0)
}

type StatusWorker struct {
	health Health
	pub    Stream
	log    log.Logger
}

func NewStatusWorker(health Health, pub Stream, logger log.Logger) *StatusWorker {
	l := logger.With("component", "StreamStatusWorker")
	return &StatusWorker{
		health: health,
		pub:    pub,
		log:    l,
	}
}

func (s *StatusWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second)

	for {
		select {
		case <-ctx.Done():
			s.log.Info("exiting StreamStatusWorker.Start", "reason", ctx.Err())
			return
		case <-ticker.C:

			status, err := s.health.Status(ctx)
			if err != nil {
				s.log.Error("failed to retrieve health status", "err", err)
				continue
			}

			s.log.Info(fmt.Sprintf("publishing %s message for replicas", status.State.String()))
			if err := s.pub.Publish(ctx, domain.SSEMessage{Event: "stream_action", Domain: status.State.String()}); err != nil {
				s.log.Error(fmt.Sprintf("failed to publish stream %s message to redis", status.State.String()), "err", err)
				continue
			}
			s.log.Info(fmt.Sprintf("successfully published %s message for replicas", status.State.String()))
		}
	}
}
