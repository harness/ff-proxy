package metricsservice

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/harness/ff-proxy/v2/stream"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/prometheus/client_golang/prometheus"
)

type key string

const (
	tokenKey key = "token"
)

// doer is a simple http client that gets passed to the generated admin client
// and injects the service token into the header before any requests are made
type doer struct {
	c     *http.Client
	token string
}

// Do injects the api-key header into the request
func (d doer) Do(r *http.Request) (*http.Response, error) {
	r.Header.Add("x-api-key", d.token)
	return d.c.Do(r)
}

type counter interface {
	prometheus.Collector
	WithLabelValues(lvs ...string) prometheus.Counter
}

// Client is a type for interacting with the Feature Flag Metric Service
type Client struct {
	log         log.Logger
	enabled     bool
	client      clientgen.ClientWithResponsesInterface
	metrics     map[string]domain.MetricsRequest
	token       string
	metricsLock *sync.Mutex
	subscriber  stream.Subscriber

	sdkUsage         counter
	metricsForwarded counter
}

// NewClient creates a MetricService
func NewClient(l log.Logger, addr string, token string, enabled bool, reg *prometheus.Registry, subscriber stream.Subscriber) (Client, error) {
	l = l.With("component", "MetricServiceClient")
	client, err := clientgen.NewClientWithResponses(
		addr,
		clientgen.WithHTTPClient(doer{c: http.DefaultClient}),
	)
	if err != nil {
		return Client{}, err
	}

	m := Client{
		log:         l,
		client:      client,
		token:       token,
		enabled:     enabled,
		metrics:     map[string]domain.MetricsRequest{},
		metricsLock: &sync.Mutex{},
		subscriber:  subscriber,

		sdkUsage: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ff_proxy_sdk_usage",
				Help: "Tracks what SDKs are using the FF Proxy",
			},
			[]string{"envID", "sdk_type", "sdk_version", "sdk_language"},
		),
		metricsForwarded: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ff_proxy_metrics_forwarded",
				Help: "Tracks the number of metrics forwarded from the Proxy to SaaS Feature Flags",
			},
			[]string{"envID", "error"},
		),
	}

	reg.MustRegister(m.sdkUsage, m.metricsForwarded)
	return m, nil
}

// StoreMetrics aggregates and stores metrics
func (c Client) StoreMetrics(_ context.Context, req domain.MetricsRequest) error {
	if !c.enabled {
		return nil
	}

	c.metricsLock.Lock()
	defer func() {
		c.trackSDKUsage(req)
		c.metricsLock.Unlock()
	}()

	// Store metrics to send later
	currentMetrics, ok := c.metrics[req.EnvironmentID]
	if !ok {
		c.metrics[req.EnvironmentID] = req
		return nil
	}

	if req.MetricsData != nil {
		if currentMetrics.MetricsData == nil {
			currentMetrics.MetricsData = &[]clientgen.MetricsData{}
		}
		newMetrics := append(*currentMetrics.MetricsData, *req.MetricsData...)
		currentMetrics.MetricsData = &newMetrics
	}

	if req.TargetData != nil {
		if currentMetrics.TargetData == nil {
			currentMetrics.TargetData = &[]clientgen.TargetData{}
		}

		newTargets := append(*currentMetrics.TargetData, *req.TargetData...)
		currentMetrics.TargetData = &newTargets
	}

	c.metrics[req.EnvironmentID] = currentMetrics
	return nil
}

// SendMetrics forwards stored metrics to the SaaS platform
func (c Client) sendMetrics(ctx context.Context, clusterIdentifier string) {
	// copy metrics before sending so we don't hog the lock for network requests
	c.metricsLock.Lock()
	metricsCopy := map[string]domain.MetricsRequest{}
	for key, val := range c.metrics {
		metricsCopy[key] = val
		delete(c.metrics, key)
	}
	c.metrics = make(map[string]domain.MetricsRequest)
	c.metricsLock.Unlock()

	for envID, metric := range metricsCopy {
		if err := c.postMetrics(ctx, envID, metric, clusterIdentifier); err != nil {
			c.log.Error("sending metrics failed", "environment", envID, "error", err)
		}
	}

}

func (c Client) postMetrics(ctx context.Context, envID string, metric domain.MetricsRequest, clusterIdentifier string) (err error) {
	defer func() {
		errLabel := "false"
		if err != nil {
			errLabel = "true"
		}
		c.metricsForwarded.WithLabelValues(envID, errLabel).Inc()
	}()

	ctx = context.WithValue(ctx, tokenKey, c.token)
	res, err := c.client.PostMetricsWithResponse(ctx, envID, &clientgen.PostMetricsParams{Cluster: &clusterIdentifier}, clientgen.PostMetricsJSONRequestBody{
		MetricsData: metric.MetricsData,
		TargetData:  metric.TargetData,
	}, addAuthToken)
	if err != nil {
		return err
	}

	if res != nil && res.StatusCode() != 200 {
		return fmt.Errorf("got non 200 status code from feature flags: status_code=%d", res.StatusCode())
	}

	return nil
}

func addAuthToken(ctx context.Context, req *http.Request) error {
	token := ctx.Value(tokenKey)
	if token == nil || token == "" {
		return fmt.Errorf("no auth token exists in context")
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	return nil
}

func (c Client) trackSDKUsage(req domain.MetricsRequest) {
	if req.MetricsData == nil {
		return
	}

	for _, me := range *req.MetricsData {
		attrMap := createAttributeMap(me.Attributes)

		sdkType := getSDKType(attrMap)
		sdkVersion := getSDKVersion(attrMap)
		sdkLanguage := getSDKLanguage(attrMap)

		c.sdkUsage.WithLabelValues(req.EnvironmentID, sdkType, sdkVersion, sdkLanguage).Inc()
	}
}

func createAttributeMap(data []clientgen.KeyValue) map[string]string {
	result := map[string]string{}
	for _, kv := range data {
		result[kv.Key] = kv.Value
	}
	return result
}

// GetSDKType returns the sdk type or an empty string if its not found
func getSDKType(m map[string]string) string {
	return m["SDK_TYPE"]
}

// GetSDKVersion returns the version or an empty string if its not found
func getSDKVersion(m map[string]string) string {
	v, ok := m["SDK_VERSION"]
	if ok {
		return v
	}

	// TODO this should be SDK_VERSION - need to update java SDK
	v2, ok := m["JAR_VERSION"]
	if ok {
		return v2
	}

	return ""
}

// GetSDKLanguage returns the language or an empty string if its not found
func getSDKLanguage(m map[string]string) string {
	return m["SDK_LANGUAGE"]
}

// PostMetrics kicks off a job that periodically sends metrics to Harness Saas if we're running in online mode and metricPostDuration is > 0
func (c Client) PostMetrics(ctx context.Context, offline bool, metricPostDuration int) {
	if offline {
		return
	}

	if metricPostDuration == 0 {
		c.log.Info("sending metrics disabled")
		return
	}

	// Start the job that listens for metrics coming in from read replicas on the redis stream
	go c.subscribe(ctx)

	c.log.Info(fmt.Sprintf("sending metrics every %d seconds", metricPostDuration))

	// start metric sending ticker
	go func() {
		ticker := time.NewTicker(time.Duration(metricPostDuration) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				c.log.Info("stopping metrics ticker")
				return
			case <-ticker.C:
				// default to prod cluster
				clusterIdentifier := "1"
				c.log.Debug("sending metrics")
				c.sendMetrics(ctx, clusterIdentifier)
			}
		}
	}()
}

// subscribe kicks off a job that subscribes to the redis stream that readreplicas write metrics onto
func (c Client) subscribe(ctx context.Context) {
	id := ""
	for {
		// There's nothing we can really do here if we error in the callback parsing/handling the message
		// so we log the errors as warnings but return nil so we carry on receiving messages from the stream
		err := c.subscriber.Sub(ctx, SDKMetricsStream, id, func(latestID string, v interface{}) error {
			// We want to keep track of the id of the latest message we've received so that if
			// we disconnect we can resume from that point
			id = latestID

			s, ok := v.(string)
			if !ok {
				c.log.Warn("unexpected message format received", "stream", SDKMetricsStream, "type", reflect.TypeOf(v))
				return nil
			}

			mr := domain.MetricsRequest{}
			if err := jsoniter.Unmarshal([]byte(s), &mr); err != nil {
				c.log.Warn("failed to unmarshal metrics message", "stream", SDKMetricsStream, "err", err)
				return nil
			}

			if err := c.StoreMetrics(ctx, mr); err != nil {
				c.log.Warn("failed to store metrics received from read replica", "stream", SDKMetricsStream, "err", err)
				return nil
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}

			c.log.Warn("dropped subscription to redis stream, backing off for 30 seconds and trying again", "stream", SDKMetricsStream, "err", err)

			// If we do break out of subscribe it will probably be because of a connection error with redis
			// so backoff for 30 seconds before trying again
			time.Sleep(30 * time.Second)
			continue
		}
	}
}
