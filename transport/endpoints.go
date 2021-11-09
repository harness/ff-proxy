package transport

import (
	"context"

	"github.com/go-kit/kit/endpoint"
	"github.com/harness/ff-proxy/domain"
)

// streamEndpoint is the endpoint definition for an endpoint that provides streaming
// functionality
type streamEndpoint func(ctx context.Context, req interface{}, stream interface{}) error

// Endpoints collects all of the endpoints that make up a ProxyService
type Endpoints struct {
	PostAuthenticate              endpoint.Endpoint
	GetFeatureConfigs             endpoint.Endpoint
	GetFeatureConfigsByIdentifier endpoint.Endpoint
	GetTargetSegments             endpoint.Endpoint
	GetTargetSegmentsByIdentifier endpoint.Endpoint
	GetEvaluations                endpoint.Endpoint
	GetEvaluationsByFeature       endpoint.Endpoint
	GetStream                     streamEndpoint
	PostMetrics                   endpoint.Endpoint
}

// NewEndpoints returns an initalised Endpoints where each endpoint invokes the
// corresponding method on the passed ProxyService
func NewEndpoints(p ProxyService) *Endpoints {
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
	}
}

// makePostAuthenticateEndpoint is a function to convert a services Authenticate
// method to an endpoint
func makePostAuthenticateEndpoint(s ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.AuthRequest)
		resp, err := s.Authenticate(ctx, req)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
}

// makeGetFeatureConfigsEndpoint is a function to convert a services GetFeatureConfig
// method to an endpoint
func makeGetFeatureConfigsEndpoint(s ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (repsonse interface{}, err error) {
		req := request.(domain.FeatureConfigRequest)
		featureConfigs, err := s.FeatureConfig(ctx, req)
		if err != nil {
			return nil, err
		}
		return featureConfigs, nil
	}
}

// makeGetFeatureConfigsByIdentifierEndpoint is a function to convert a services
// FeatureConfigByIdentifier method to an endpoint
func makeGetFeatureConfigsByIdentifierEndpoint(s ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.FeatureConfigByIdentifierRequest)
		featureConfig, err := s.FeatureConfigByIdentifier(ctx, req)
		if err != nil {
			return nil, err
		}
		return featureConfig, nil
	}
}

// makeGetTargetSegmentsEndpoint is a function to convert a services TargetSegments
// method to an endpoint
func makeGetTargetSegmentsEndpoint(s ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.TargetSegmentsRequest)
		segments, err := s.TargetSegments(ctx, req)
		if err != nil {
			return nil, err
		}
		return segments, nil
	}
}

// makeGetTargetSegmentsByIdentifierEndpoint is a function to convert a services
// TargetSegmentsByIdentifier method to an endpoint
func makeGetTargetSegmentsByIdentifierEndpoint(s ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.TargetSegmentsByIdentifierRequest)
		segment, err := s.TargetSegmentsByIdentifier(ctx, req)
		if err != nil {
			return nil, err
		}
		return segment, nil
	}
}

// makeGetEvaluationsEndpoint is a function to convert a services Evaluations
// method to an endpoint
func makeGetEvaluationsEndpoint(s ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.EvaluationsRequest)
		evaluation, err := s.Evaluations(ctx, req)
		if err != nil {
			return nil, err
		}
		return evaluation, nil
	}
}

// makeGetEvaluationsByFeatureEndpoint is a function to convert a services
// EvaluationsByFeature method to an endpoint
func makeGetEvaluationsByFeatureEndpoint(s ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.EvaluationsByFeatureRequest)
		evaluations, err := s.EvaluationsByFeature(ctx, req)
		if err != nil {
			return nil, err
		}
		return evaluations, nil
	}
}

// makeGetStreamEndpoint is a function to convert a services Stream method
// to an endpoint
func makeGetStreamEndpoint(s ProxyService) streamEndpoint {
	return func(ctx context.Context, request interface{}, stream interface{}) (err error) {
		req := request.(domain.StreamRequest)
		wstream := stream.(domain.Stream)

		err = s.Stream(ctx, req, wstream)
		if err != nil {
			return err
		}
		return nil
	}
}

// makePostMetricsEndpoint is a function to convert a services Metrics method
// to an endpoint
func makePostMetricsEndpoint(s ProxyService) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(domain.MetricsRequest)
		if err := s.Metrics(ctx, req); err != nil {
			return nil, err
		}
		return nil, nil
	}
}
