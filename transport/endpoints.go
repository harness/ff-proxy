package transport

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/harness/ff-proxy/v2/domain"
	proxyservice "github.com/harness/ff-proxy/v2/proxy-service"
)

// Endpoints collects all of the endpoints that make up a ProxyService
type Endpoints struct {
	PostAuthenticate              endpoint.Endpoint
	GetFeatureConfigs             endpoint.Endpoint
	GetFeatureConfigsByIdentifier endpoint.Endpoint
	GetTargetSegments             endpoint.Endpoint
	GetTargetSegmentsByIdentifier endpoint.Endpoint
	GetEvaluations                endpoint.Endpoint
	GetEvaluationsByFeature       endpoint.Endpoint
	GetStream                     endpoint.Endpoint
	PostMetrics                   endpoint.Endpoint
	Health                        endpoint.Endpoint
}

// NewEndpoints returns an initialised Endpoints where each endpoint invokes the
// corresponding method on the passed ProxyService
func NewEndpoints(p proxyservice.ProxyService) *Endpoints {
	return &Endpoints{
		PostAuthenticate:              makePostAuthenticateEndpoint(p),
		GetFeatureConfigs:             makeGetFeatureConfigsEndpoint(p),
		GetFeatureConfigsByIdentifier: makeGetFeatureConfigsByIdentifierEndpoint(p),
		GetTargetSegments:             makeGetTargetSegmentsEndpoint(p),
		GetTargetSegmentsByIdentifier: makeGetTargetSegmentsByIdentifierEndpoint(p),
		GetEvaluations:                makeGetEvaluationsEndpoint(p),
		GetEvaluationsByFeature:       makeGetEvaluationsByFeatureEndpoint(p),
		GetStream:                     makeGetStreamEndpoint(p),
		PostMetrics:                   makePostMetricsEndpoint(p),
		Health:                        makeHealthEndpoint(p),
	}
}

// makePostAuthenticateEndpoint is a function to convert a clients Authenticate
// method to an endpoint
func makePostAuthenticateEndpoint(s proxyservice.ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.AuthRequest)
		resp, err := s.Authenticate(ctx, req)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
}

// makeGetFeatureConfigsEndpoint is a function to convert a clients GetFeatureConfig
// method to an endpoint
func makeGetFeatureConfigsEndpoint(s proxyservice.ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.FeatureConfigRequest)
		featureConfigs, err := s.FeatureConfig(ctx, req)
		if err != nil {
			return nil, err
		}
		return featureConfigs, nil
	}
}

// makeGetFeatureConfigsByIdentifierEndpoint is a function to convert a clients
// FeatureConfigByIdentifier method to an endpoint
func makeGetFeatureConfigsByIdentifierEndpoint(s proxyservice.ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.FeatureConfigByIdentifierRequest)
		featureConfig, err := s.FeatureConfigByIdentifier(ctx, req)
		if err != nil {
			return nil, err
		}
		return featureConfig, nil
	}
}

// makeGetTargetSegmentsEndpoint is a function to convert a clients TargetSegments
// method to an endpoint
func makeGetTargetSegmentsEndpoint(s proxyservice.ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.TargetSegmentsRequest)
		segments, err := s.TargetSegments(ctx, req)
		if err != nil {
			return nil, err
		}
		return segments, nil
	}
}

// makeGetTargetSegmentsByIdentifierEndpoint is a function to convert a clients
// TargetSegmentsByIdentifier method to an endpoint
func makeGetTargetSegmentsByIdentifierEndpoint(s proxyservice.ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.TargetSegmentsByIdentifierRequest)
		segment, err := s.TargetSegmentsByIdentifier(ctx, req)
		if err != nil {
			return nil, err
		}
		return segment, nil
	}
}

// makeGetEvaluationsEndpoint is a function to convert a clients Evaluations
// method to an endpoint
func makeGetEvaluationsEndpoint(s proxyservice.ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.EvaluationsRequest)
		evaluation, err := s.Evaluations(ctx, req)
		if err != nil {
			return nil, err
		}
		return evaluation, nil
	}
}

// makeGetEvaluationsByFeatureEndpoint is a function to convert a clients
// EvaluationsByFeature method to an endpoint
func makeGetEvaluationsByFeatureEndpoint(s proxyservice.ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.EvaluationsByFeatureRequest)
		evaluations, err := s.EvaluationsByFeature(ctx, req)
		if err != nil {
			return nil, err
		}
		return evaluations, nil
	}
}

// makeGetStreamEndpoint is a function to convert a clients Stream method
// to an endpoint
func makeGetStreamEndpoint(s proxyservice.ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.StreamRequest)
		resp, err := s.Stream(ctx, req)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
}

// makePostMetricsEndpoint is a function to convert a clients Metrics method
// to an endpoint
func makePostMetricsEndpoint(s proxyservice.ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.MetricsRequest)
		if err := s.Metrics(ctx, req); err != nil {
			return nil, err
		}
		return nil, nil
	}
}

// makeHealthEndpoint is a function to convert a clients Health method
// to an endpoint
func makeHealthEndpoint(s proxyservice.ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		res, err := s.Health(ctx)
		if err != nil {
			return nil, err
		}
		return res, nil
	}
}
