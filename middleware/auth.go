package middleware

import (
	"context"

	"github.com/harness/ff-proxy/domain"
	clientgen "github.com/harness/ff-proxy/gen/client"
	proxyservice "github.com/harness/ff-proxy/proxy-service"
)

// tokenValidator is a function that can validate a jwt token
type tokenValidator func(token string) bool

// AuthMiddleware is an applicaiton middleware that wraps a ProxyService and
// applies authentication to its endpoints.
type AuthMiddleware struct {
	next       proxyservice.ProxyService
	bypassAuth bool
	validToken tokenValidator
}

// NewAuthMiddleware creates a new AuthMiddleware, passing true for the bypassAuth param
// will bypass authentication entirely on all endpoints.
func NewAuthMiddleware(validator tokenValidator, bypassAuth bool, next proxyservice.ProxyService) proxyservice.ProxyService {
	return AuthMiddleware{next: next, bypassAuth: bypassAuth, validToken: validator}
}

// Authenticate calls the wrapped services Authenticate method and returns the response unaltered
func (a AuthMiddleware) Authenticate(ctx context.Context, req domain.AuthRequest) (domain.AuthResponse, error) {
	return a.next.Authenticate(ctx, req)
}

// FeatureConfig checks that the auth token is valid and then calls the wrapped
// services FeatureConfig method. If auth is bypassed then it just goes straight
// to calling the wrapped service.
func (a AuthMiddleware) FeatureConfig(ctx context.Context, req domain.FeatureConfigRequest) ([]domain.FeatureConfig, error) {
	if a.bypassAuth {
		return a.next.FeatureConfig(ctx, req)
	}

	if !a.validToken(req.Token) {
		return []domain.FeatureConfig{}, proxyservice.ErrUnauthorised
	}
	return a.next.FeatureConfig(ctx, req)
}

// FeatureConfigByIdentifier checks that the auth token is valid and then calls
// the wrapped services FeatureConfigByIdentifier method. If auth is bypassed
// then it just goes straight to calling the wrapped service.
func (a AuthMiddleware) FeatureConfigByIdentifier(ctx context.Context, req domain.FeatureConfigByIdentifierRequest) (domain.FeatureConfig, error) {
	if a.bypassAuth {
		return a.next.FeatureConfigByIdentifier(ctx, req)
	}

	if !a.validToken(req.Token) {
		return domain.FeatureConfig{}, proxyservice.ErrUnauthorised
	}

	return a.next.FeatureConfigByIdentifier(ctx, req)
}

// TargetSegments checks that the auth token is valid and then calls the wrapped
// services TargetSegments method. If auth is bypassed then it just goes straight
// to calling the wrapped service.
func (a AuthMiddleware) TargetSegments(ctx context.Context, req domain.TargetSegmentsRequest) ([]domain.Segment, error) {
	if a.bypassAuth {
		return a.next.TargetSegments(ctx, req)
	}

	if !a.validToken(req.Token) {
		return []domain.Segment{}, proxyservice.ErrUnauthorised
	}

	return a.next.TargetSegments(ctx, req)
}

// TargetSegmentsByIdentifier checks that the auth token is valid and then calls
// the wrapped services TargetSegmentsByIdentifier method. If auth is bypassed
// then it just goes straight to calling the wrapped service.
func (a AuthMiddleware) TargetSegmentsByIdentifier(ctx context.Context, req domain.TargetSegmentsByIdentifierRequest) (domain.Segment, error) {
	if a.bypassAuth {
		return a.next.TargetSegmentsByIdentifier(ctx, req)
	}

	if !a.validToken(req.Token) {
		return domain.Segment{}, proxyservice.ErrUnauthorised
	}
	return a.next.TargetSegmentsByIdentifier(ctx, req)
}

// Evaluations checks that the auth token is valid and then calls the wrapped
// services TargetSegmentsByIdentifier method. If auth is bypassed then it just
// goes straight to calling the wrapped service.
func (a AuthMiddleware) Evaluations(ctx context.Context, req domain.EvaluationsRequest) ([]clientgen.Evaluation, error) {
	if a.bypassAuth {
		return a.next.Evaluations(ctx, req)
	}

	if !a.validToken(req.Token) {
		return []clientgen.Evaluation{}, proxyservice.ErrUnauthorised
	}
	return a.next.Evaluations(ctx, req)
}

// EvaluationsByFeature checks that the auth token is valid and then calls the
// wrapped services EvaluationsByFeature method. If auth is bypassed then it just
// goes straight to calling the wrapped service.
func (a AuthMiddleware) EvaluationsByFeature(ctx context.Context, req domain.EvaluationsByFeatureRequest) (clientgen.Evaluation, error) {
	if a.bypassAuth {
		return a.next.EvaluationsByFeature(ctx, req)
	}

	if !a.validToken(req.Token) {
		return clientgen.Evaluation{}, proxyservice.ErrUnauthorised
	}
	return a.next.EvaluationsByFeature(ctx, req)
}

// Stream checks that the auth token is valid and then calls the wrapped services
// Stream method. If auth is bypassed then it just goes straight to calling the
// wrapped service.
func (a AuthMiddleware) Stream(ctx context.Context, req domain.StreamRequest, stream domain.Stream) error {
	if a.bypassAuth {
		return a.next.Stream(ctx, req, stream)
	}

	if !a.validToken(req.Token) {
		return proxyservice.ErrUnauthorised
	}
	return a.next.Stream(ctx, req, stream)
}

// Metrics checks that the auth token is valid and then calls the wrapped services
// Metrics method. If auth is bypassed then it just goes straight to calling the
// wrapped service.
func (a AuthMiddleware) Metrics(ctx context.Context, req domain.MetricsRequest) error {
	if a.bypassAuth {
		return a.next.Metrics(ctx, req)
	}

	if !a.validToken(req.Token) {
		return proxyservice.ErrUnauthorised
	}
	return a.next.Metrics(ctx, req)
}
