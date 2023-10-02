package services

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

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

// MetricService is a type for interacting with the Feature Flag Metric Service
type MetricService struct {
	log         log.Logger
	enabled     bool
	client      clientgen.ClientWithResponsesInterface
	metrics     map[string]domain.MetricsRequest
	token       string
	metricsLock *sync.Mutex

	sdkUsage         counter
	metricsForwarded counter
}

// NewMetricService creates a MetricService
func NewMetricService(l log.Logger, addr string, token string, enabled bool, reg *prometheus.Registry) (MetricService, error) {
	l = l.With("component", "MetricServiceClient")
	client, err := clientgen.NewClientWithResponses(
		addr,
		clientgen.WithHTTPClient(doer{c: http.DefaultClient}),
	)
	if err != nil {
		return MetricService{}, err
	}

	m := MetricService{
		log:         l,
		client:      client,
		token:       token,
		enabled:     enabled,
		metrics:     map[string]domain.MetricsRequest{},
		metricsLock: &sync.Mutex{},

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
func (m MetricService) StoreMetrics(req domain.MetricsRequest) error {
	if !m.enabled {
		return nil
	}

	m.metricsLock.Lock()
	defer func() {
		m.trackSDKUsage(req)
		m.metricsLock.Unlock()
	}()

	// Store metrics to send later
	currentMetrics, ok := m.metrics[req.EnvironmentID]
	if !ok {
		m.metrics[req.EnvironmentID] = req
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

	m.metrics[req.EnvironmentID] = currentMetrics
	return nil
}

// SendMetrics forwards stored metrics to the SaaS platform
func (m MetricService) sendMetrics(ctx context.Context, clusterIdentifier string) {
	// copy metrics before sending so we don't hog the lock for network requests
	m.metricsLock.Lock()
	metricsCopy := map[string]domain.MetricsRequest{}
	for key, val := range m.metrics {
		metricsCopy[key] = val
		delete(m.metrics, key)
	}
	m.metrics = make(map[string]domain.MetricsRequest)
	m.metricsLock.Unlock()

	for envID, metric := range metricsCopy {
		if err := m.postMetrics(ctx, envID, metric, clusterIdentifier); err != nil {
			m.log.Error("sending metrics failed", "environment", envID, "error", err)
		}
	}

}

func (m MetricService) postMetrics(ctx context.Context, envID string, metric domain.MetricsRequest, clusterIdentifier string) (err error) {
	defer func() {
		errLabel := "false"
		if err != nil {
			errLabel = "true"
		}
		m.metricsForwarded.WithLabelValues(envID, errLabel).Inc()
	}()

	ctx = context.WithValue(ctx, tokenKey, m.token)
	res, err := m.client.PostMetricsWithResponse(ctx, envID, &clientgen.PostMetricsParams{Cluster: &clusterIdentifier}, clientgen.PostMetricsJSONRequestBody{
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

func (m MetricService) trackSDKUsage(req domain.MetricsRequest) {
	if req.MetricsData == nil {
		return
	}

	for _, me := range *req.MetricsData {
		attrMap := createAttributeMap(me.Attributes)

		sdkType := getSDKType(attrMap)
		sdkVersion := getSDKVersion(attrMap)
		sdkLanguage := getSDKLanguage(attrMap)

		m.sdkUsage.WithLabelValues(req.EnvironmentID, sdkType, sdkVersion, sdkLanguage).Inc()
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

// Send periodically sends metrics to Harness Saas if we're running in online mode and metricPostDuration is > 0
func (m MetricService) Send(ctx context.Context, offline bool, metricPostDuration int) {
	if offline {
		return
	}

	if metricPostDuration == 0 {
		m.log.Info("sending metrics disabled")
		return
	}

	m.log.Info(fmt.Sprintf("sending metrics every %d seconds", metricPostDuration))

	// start metric sending ticker
	go func() {
		ticker := time.NewTicker(time.Duration(metricPostDuration) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				m.log.Info("stopping metrics ticker")
				return
			case <-ticker.C:
				// default to prod cluster
				clusterIdentifier := "1"
				m.log.Debug("sending metrics")
				m.sendMetrics(ctx, clusterIdentifier)
			}
		}
	}()
}
