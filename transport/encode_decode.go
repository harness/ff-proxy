package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/harness/ff-proxy/domain"
	proxyservice "github.com/harness/ff-proxy/proxy-service"
)

var (
	errBadRouting = errors.New("bad routing")
	errBadRequest = errors.New("bad request")
)

// encodeResponse is the common method to encode all the non error response types
// to the client. If we need to we can write specific encodeResponse functions
// for endpoints that require one.
func encodeResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}

// encodeError encodes error responses returned from handlers
func encodeError(ctx context.Context, err error, w http.ResponseWriter) {
	if err == nil {
		panic("encodeError with nil error")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codeFrom(err))
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err.Error(),
	})
}

// encodeStreamResponse sets the headers for a streaming response
func encodeStreamResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	return nil
}

// codeFrom casts a service error to an http.StatusCode
func codeFrom(err error) int {
	switch err {
	case proxyservice.ErrNotImplemented:
		return http.StatusNotImplemented
	case proxyservice.ErrNotFound:
		return http.StatusNotFound
	default:
		if errors.Is(err, errBadRequest) {
			return http.StatusBadRequest
		}
		return http.StatusInternalServerError
	}
}

// decodeAuthRequest decodes POST /client/auth requests into a domain.AuthRequest
// that can be passed to the service. It returns a wrapped bad request error if
// the request body is empty or if the apiKey is empty
func decodeAuthRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	req := domain.AuthRequest{}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	if len(b) == 0 {
		return nil, fmt.Errorf("%w: request body cannot be empty", errBadRequest)
	}

	if err := json.Unmarshal(b, &req); err != nil {
		return nil, err
	}

	if req.APIKey == "" {
		return nil, fmt.Errorf("%w: apiKey cannot be empty", errBadRequest)
	}
	return req, nil
}

// decodeGetFeatureConfigisRequest decodes GET /client/env/{environmentUUID}/feature-configs requests
// into a domain.FeatureConfigRequest that can be passed to the ProxyService
func decodeGetFeatureConfigsRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	envID, ok := vars["environmentUUID"]
	if !ok {
		return nil, errBadRouting
	}

	req := domain.FeatureConfigRequest{
		EnvironmentID: envID,
	}
	return req, nil
}

// decodeGetFeatureConfigsByIdentifierRequest decodes GET /client/env/{environmentUUID}/feature-configs/{identifier} requests
// into a domain.FeatureConfigsByIdentifierRequest that can be passed to the ProxyService
func decodeGetFeatureConfigsByIdentifierRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	envID, ok := vars["environmentUUID"]
	if !ok {
		return nil, errBadRouting
	}

	identifier := vars["identifier"]
	req := domain.FeatureConfigByIdentifierRequest{
		EnvironmentID: envID,
		Identifier:    identifier,
	}
	return req, nil
}

// decodeGetTargetSegmentsRequest decodes GET /client/env/{environmentUUID}/target-segments requests
// into a domain.TargetSegmentsRequest that can be passed to the ProxyService
func decodeGetTargetSegmentsRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	envID, ok := vars["environmentUUID"]
	if !ok {
		return nil, errBadRouting
	}

	req := domain.TargetSegmentsRequest{
		EnvironmentID: envID,
	}
	return req, nil
}

// decodeGetTargetSegmentsByIdentifierRequest decodes GET /client/env/{environmentUUID}/target-segments/{identifier}
// requests into a domain.TargetSegmentsByIdentifierRequest that can be passed to the ProxyService
func decodeGetTargetSegmentsByIdentifierRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	envID, ok := vars["environmentUUID"]
	if !ok {
		return nil, errBadRouting
	}

	identifier := vars["identifier"]
	if !ok {
		return nil, errBadRouting
	}

	req := domain.TargetSegmentsByIdentifierRequest{
		EnvironmentID: envID,
		Identifier:    identifier,
	}
	return req, nil
}

// decodeGetEvaluationsRequest decodes GET /client/env/{environmentUUID}/target/{target}/evaluations
// requests into a domain.EvaluationsRequest that can be passed to the ProxyService
func decodeGetEvaluationsRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)

	envID, ok := vars["environmentUUID"]
	if !ok {
		return nil, errBadRouting
	}

	target, ok := vars["target"]
	if !ok {
		return nil, errBadRouting
	}

	req := domain.EvaluationsRequest{
		EnvironmentID:    envID,
		TargetIdentifier: target,
	}
	return req, nil
}

// decodeGetEvaluationsByFeatureRequest decodes GET /client/env/{environmentUUID}/target/{target}/evaluations/{feature}
// requests into a domain.EvaluationsByFeatureRequest that can be passed to the ProxyService
func decodeGetEvaluationsByFeatureRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)

	envID, ok := vars["environmentUUID"]
	if !ok {
		return nil, errBadRouting
	}

	target, ok := vars["target"]
	if !ok {
		return nil, errBadRouting
	}

	feature, ok := vars["feature"]
	if !ok {
		return nil, errBadRouting
	}

	req := domain.EvaluationsByFeatureRequest{
		EnvironmentID:     envID,
		TargetIdentifier:  target,
		FeatureIdentifier: feature,
	}
	return req, nil
}

// decodeGetStreamRequest decodes GET /stream requests into a domain.StreamRequest that
// can be passed to the ProxyService
func decodeGetStreamRequest(ctx context.Context, r *http.Request) (interface{}, error) {
	apiKey := r.Header.Get("API-Key")

	req := domain.StreamRequest{
		APIKey: apiKey,
	}

	if req.APIKey == "" {
		return nil, fmt.Errorf("%w: API-Key can't be empty", errBadRequest)
	}
	return req, nil
}
