package transport

import (
	"net/http"

	"github.com/go-kit/kit/transport"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/go-kit/log"
	"github.com/harness/ff-proxy/domain"
)

// StreamHandlerOption sets an optional parameter for a StreamHandler
type StreamHandlerOption func(s *StreamHandler)

// StreamHandlerBefore sets RequestFuncs in the StreamHandler that are executed before
// the request is decoded
func StreamHandlerBefore(before ...httptransport.RequestFunc) StreamHandlerOption {
	return func(s *StreamHandler) {
		s.before = append(s.before, before...)
	}
}

// StreamHandlerErrorEncoder sets the ErrorEncoder used by the StreamHandler
func StreamHandlerErrorEncoder(ee httptransport.ErrorEncoder) StreamHandlerOption {
	return func(s *StreamHandler) {
		s.errorEncoder = ee
	}
}

// StreamHandlerErrorHandler sets the ErrorHandler used by the StreamHandler
func StreamHandlerErrorHandler(ee transport.ErrorHandler) StreamHandlerOption {
	return func(s *StreamHandler) {
		s.errorHandler = ee
	}
}

// StreamHandler is the handler used for handling streaming requests
type StreamHandler struct {
	endpoint     streamEndpoint
	dec          httptransport.DecodeRequestFunc
	enc          httptransport.EncodeResponseFunc
	before       []httptransport.RequestFunc
	errorEncoder httptransport.ErrorEncoder
	errorHandler transport.ErrorHandler
}

// NewStreamHandler creates and returns a StreamHandler
func NewStreamHandler(e streamEndpoint, dec httptransport.DecodeRequestFunc, enc httptransport.EncodeResponseFunc, opts ...StreamHandlerOption) StreamHandler {
	s := StreamHandler{
		endpoint:     e,
		dec:          dec,
		enc:          enc,
		errorEncoder: httptransport.DefaultErrorEncoder,
		errorHandler: transport.NewLogErrorHandler(log.NewNopLogger()),
	}

	for _, opt := range opts {
		opt(&s)
	}
	return s
}

// ServeHTTP makes stream handler implement http.Handler
func (s StreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	for _, fn := range s.before {
		ctx = fn(ctx, r)
	}

	req, err := s.dec(ctx, r)
	if err != nil {
		s.errorHandler.Handle(ctx, err)
		s.errorEncoder(ctx, err, w)
		return
	}

	stream := domain.NewStream(w)
	err = s.endpoint(ctx, req, stream)
	if err != nil {
		s.errorHandler.Handle(ctx, err)
		s.errorEncoder(ctx, err, w)
		return
	}

	if err := s.enc(ctx, w, stream); err != nil {
		s.errorHandler.Handle(ctx, err)
		s.errorEncoder(ctx, err, w)
		return
	}
}
