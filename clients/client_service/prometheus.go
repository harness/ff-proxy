package clientservice

import (
	"context"
	"strconv"
	"time"

	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/prometheus/client_golang/prometheus"
)

// prometheusClient is used to decorating an ffClientService implementation that tracks prometheus metrics
type prometheusClient struct {
	requestCount    *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec

	next ffClientService
}

func newPrometheusClient(next ffClientService, reg *prometheus.Registry) prometheusClient {
	p := prometheusClient{
		requestCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ff_proxy_to_client_service_requests",
			Help: "Tracks the number of requests that the Proxy makes to the ff-client-service in Harness Saas",
		},
			[]string{"url", "envID", "code"},
		),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "ff_proxy_to_ff_client_service_requests_duration",
			Help:    "Tracks the request duration for requests made from the ff-proxy to the ff-client service",
			Buckets: []float64{0.05, 0.1, 0.2, 0.3, 0.4, 0.5, 0.75, 1},
		},
			[]string{"url", "envID"},
		),
		next: next,
	}

	reg.MustRegister(p.requestCount, p.requestDuration)

	return p
}

func (p prometheusClient) AuthenticateWithResponse(ctx context.Context, body clientgen.AuthenticateJSONRequestBody, reqEditors ...clientgen.RequestEditorFn) (resp *clientgen.AuthenticateResponse, err error) {
	start := time.Now()
	defer func() {
		p.requestCount.WithLabelValues("/client/auth", "", strconv.Itoa(resp.StatusCode())).Inc()
		p.requestDuration.WithLabelValues("/client/auth", "").Observe(time.Since(start).Seconds())
	}()

	return p.next.AuthenticateWithResponse(ctx, body, reqEditors...)
}

func (p prometheusClient) AuthenticateProxyKeyWithResponse(ctx context.Context, body clientgen.AuthenticateProxyKeyJSONRequestBody, reqEditors ...clientgen.RequestEditorFn) (resp *clientgen.AuthenticateProxyKeyResponse, err error) {
	start := time.Now()
	defer func() {
		p.requestCount.WithLabelValues("/proxy/auth", "", strconv.Itoa(resp.StatusCode())).Inc()
		p.requestDuration.WithLabelValues("/proxy/auth", "").Observe(time.Since(start).Seconds())
	}()

	return p.next.AuthenticateProxyKeyWithResponse(ctx, body, reqEditors...)
}

func (p prometheusClient) GetProxyConfigWithResponse(ctx context.Context, params *clientgen.GetProxyConfigParams, reqEditors ...clientgen.RequestEditorFn) (resp *clientgen.GetProxyConfigResponse, err error) {
	start := time.Now()
	defer func() {
		p.requestCount.WithLabelValues("/proxy/config", "", strconv.Itoa(resp.StatusCode())).Inc()
		p.requestDuration.WithLabelValues("/proxy/config", "").Observe(time.Since(start).Seconds())
	}()

	return p.next.GetProxyConfigWithResponse(ctx, params, reqEditors...)
}

func (p prometheusClient) GetAllSegmentsWithResponse(ctx context.Context, environmentUUID string, params *clientgen.GetAllSegmentsParams, reqEditors ...clientgen.RequestEditorFn) (resp *clientgen.GetAllSegmentsResponse, err error) {
	start := time.Now()
	defer func() {
		p.requestCount.WithLabelValues("/client/env/:env/target-segments", environmentUUID, strconv.Itoa(resp.StatusCode())).Inc()
		p.requestDuration.WithLabelValues("/client/env/:env/target-segments", environmentUUID).Observe(time.Since(start).Seconds())
	}()

	return p.next.GetAllSegmentsWithResponse(ctx, environmentUUID, params, reqEditors...)
}

func (p prometheusClient) GetFeatureConfigWithResponse(ctx context.Context, environmentUUID string, params *clientgen.GetFeatureConfigParams, reqEditors ...clientgen.RequestEditorFn) (resp *clientgen.GetFeatureConfigResponse, err error) {
	start := time.Now()
	defer func() {
		p.requestCount.WithLabelValues("/client/env/:env/feature-configs", environmentUUID, strconv.Itoa(resp.StatusCode())).Inc()
		p.requestDuration.WithLabelValues("/client/env/:env/feature-configs", environmentUUID).Observe(time.Since(start).Seconds())
	}()

	return p.next.GetFeatureConfigWithResponse(ctx, environmentUUID, params, reqEditors...)
}
