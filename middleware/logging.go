package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/harness/ff-proxy/domain"
	clientgen "github.com/harness/ff-proxy/gen/client"
	"github.com/harness/ff-proxy/log"
	proxyservice "github.com/harness/ff-proxy/proxy-service"
)

const (
	redactedToken = "xxxx-xxxx-xxxx-xxxx"
)

// LoggingMiddleware is an application middleware that wraps a ProxyService and
// logs out the method name, parameters, response and the duration. Note it only
// logs the response if debug is enabled as this can be quite large
type LoggingMiddleware struct {
	logger log.Logger
	debug  bool
	next   proxyservice.ProxyService
}

// NewLoggingMiddleware creates a new LoggingMiddleware
func NewLoggingMiddleware(l log.Logger, debug bool, next proxyservice.ProxyService) proxyservice.ProxyService {
	l = log.With(l, "component", "LoggingMiddleware")
	return LoggingMiddleware{
		logger: l,
		debug:  debug,
		next:   next,
	}
}

// Authenticate performs logging on authenticate requests and logs out the method, request parameters, error and duration.
// If debug is enabled it will also log out the response
func (l LoggingMiddleware) Authenticate(ctx context.Context, req domain.AuthRequest) (resp domain.AuthResponse, err error) {
	defer func(begin time.Time) {
		if l.debug {
			l.logger.Debug(
				"method", "Authenticate",
				"input", fmt.Sprintf("%+v", req),
				"output", fmt.Sprintf("%+v", resp),
				"err", err,
				"took", time.Since(begin),
			)
		} else {
			l.logger.Info(
				"method", "Authenticate",
				"input", fmt.Sprintf("%+v", req),
				"err", err,
				"took", time.Since(begin),
			)
		}
	}(time.Now())

	resp, err = l.next.Authenticate(ctx, req)
	return
}

// FeatureConfig performs logging on FeatureConfig requests and logs out the method, request parameters, error and duration.
// If debug is enabled it will also log out the response
func (l LoggingMiddleware) FeatureConfig(ctx context.Context, req domain.FeatureConfigRequest) (resp []domain.FeatureConfig, err error) {
	defer func(begin time.Time) {
		req.Token = redactedToken
		if l.debug {
			l.logger.Debug(
				"method", "FeatureConfig",
				"input", fmt.Sprintf("%+v", req),
				"output", fmt.Sprintf("%+v", resp),
				"err", err,
				"took", time.Since(begin),
			)
		} else {
			l.logger.Info(
				"method", "FeatureConfig",
				"input", fmt.Sprintf("%+v", req),
				"err", err,
				"took", time.Since(begin),
			)
		}
	}(time.Now())

	resp, err = l.next.FeatureConfig(ctx, req)
	return
}

// FeatureConfigByIdentifier performs logging on FeatureConfig requests and logs out the method, request parameters, error and duration.
// If debug is enabled it will also log out the response
func (l LoggingMiddleware) FeatureConfigByIdentifier(ctx context.Context, req domain.FeatureConfigByIdentifierRequest) (resp domain.FeatureConfig, err error) {
	defer func(begin time.Time) {
		req.Token = redactedToken
		if l.debug {
			l.logger.Debug(
				"method", "FeatureConfigByIdentifier",
				"input", fmt.Sprintf("%+v", req),
				"output", fmt.Sprintf("%+v", resp),
				"err", err,
				"took", time.Since(begin),
			)
		} else {
			l.logger.Info(
				"method", "FeatureConfigByIdentifier",
				"input", fmt.Sprintf("%+v", req),
				"err", err,
				"took", time.Since(begin),
			)
		}
	}(time.Now())

	resp, err = l.next.FeatureConfigByIdentifier(ctx, req)
	return
}

// TargetSegments performs logging on TargetSegments requests and logs out the method, request parameters, error and duration.
// If debug is enabled it will also log out the response
func (l LoggingMiddleware) TargetSegments(ctx context.Context, req domain.TargetSegmentsRequest) (resp []domain.Segment, err error) {
	defer func(begin time.Time) {
		req.Token = redactedToken
		if l.debug {
			l.logger.Debug(
				"method", "TargetSegments",
				"input", fmt.Sprintf("%+v", req),
				"output", fmt.Sprintf("%+v", resp),
				"err", err,
				"took", time.Since(begin),
			)
		} else {
			l.logger.Info(
				"method", "TargetSegments",
				"input", fmt.Sprintf("%+v", req),
				"err", err,
				"took", time.Since(begin),
			)
		}
	}(time.Now())

	resp, err = l.next.TargetSegments(ctx, req)
	return
}

// TargetSegmentsByIdentifier performs logging on TargetSegmentsByIdentifer requests
// and logs out the method, request parameters, error and duration. If debug is
// enabled it will also log out the response
func (l LoggingMiddleware) TargetSegmentsByIdentifier(ctx context.Context, req domain.TargetSegmentsByIdentifierRequest) (resp domain.Segment, err error) {
	defer func(begin time.Time) {
		req.Token = redactedToken
		if l.debug {
			l.logger.Debug(
				"method", "TargetSegmentsByIdentifier",
				"input", fmt.Sprintf("%+v", req),
				"output", fmt.Sprintf("%+v", resp),
				"err", err,
				"took", time.Since(begin),
			)
		} else {
			l.logger.Info(
				"method", "TargetSegmentsByIdentifier",
				"input", fmt.Sprintf("%+v", req),
				"err", err,
				"took", time.Since(begin),
			)
		}
	}(time.Now())

	resp, err = l.next.TargetSegmentsByIdentifier(ctx, req)
	return
}

// Evaluations performs logging on Evaluations requests and logs out the method,
// request parameters, error and duration. If debug is enabled it will also log
// out the response
func (l LoggingMiddleware) Evaluations(ctx context.Context, req domain.EvaluationsRequest) (resp []clientgen.Evaluation, err error) {
	defer func(begin time.Time) {
		req.Token = redactedToken
		if l.debug {
			l.logger.Debug(
				"method", "Evaluations",
				"input", fmt.Sprintf("%+v", req),
				"output", fmt.Sprintf("%+v", resp),
				"err", err,
				"took", time.Since(begin),
			)
		} else {
			l.logger.Info(
				"method", "Evaluations",
				"input", fmt.Sprintf("%+v", req),
				"err", err,
				"took", time.Since(begin),
			)
		}
	}(time.Now())

	resp, err = l.next.Evaluations(ctx, req)
	return
}

// EvaluationsByFeature performs logging on EvaluationsByFeature requests and
// logs out the method, request parameters, error and duration. If debug is enabled
// it will also log out the response
func (l LoggingMiddleware) EvaluationsByFeature(ctx context.Context, req domain.EvaluationsByFeatureRequest) (resp clientgen.Evaluation, err error) {
	defer func(begin time.Time) {
		req.Token = redactedToken
		if l.debug {
			l.logger.Debug(
				"method", "EvaluationsByFeature",
				"input", fmt.Sprintf("%+v", req),
				"output", fmt.Sprintf("%+v", resp),
				"err", err,
				"took", time.Since(begin),
			)
		} else {
			l.logger.Info(
				"method", "EvaluationsByFeature",
				"input", fmt.Sprintf("%+v", req),
				"err", err,
				"took", time.Since(begin),
			)
		}
	}(time.Now())

	resp, err = l.next.EvaluationsByFeature(ctx, req)
	return
}

// Stream performs logging on Stream requests and logs out the method, request
// parameters, error and duration.
func (l LoggingMiddleware) Stream(ctx context.Context, req domain.StreamRequest, stream domain.Stream) (err error) {
	defer func(begin time.Time) {
		req.Token = redactedToken
		l.logger.Info(
			"method", "Stream",
			"input", fmt.Sprintf("%+v", req),
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())

	err = l.next.Stream(ctx, req, stream)
	return
}

// Metrics performs logging on Metrics requests and logs out the method, request
// parameters, error and duration.
func (l LoggingMiddleware) Metrics(ctx context.Context, req domain.MetricsRequest) (err error) {
	defer func(begin time.Time) {
		req.Token = redactedToken
		l.logger.Info(
			"method", "Metrics",
			//"input", fmt.Sprintf("%+v", req),
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())

	err = l.next.Metrics(ctx, req)
	return
}
