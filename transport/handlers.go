package transport

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/go-kit/kit/endpoint"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/labstack/echo/v4"
)

// errorEncoderFunc is a function for encoding errors and writing
// error responses
type errorEncoderFunc func(c echo.Context, err error) error

// decodeRequestFunc is a function that decodes http requests into a type
type decodeRequestFunc func(c echo.Context, l log.Logger) (request interface{}, err error)

// encodeResponseFunc is a function for encoding http responses
type encodeResponseFunc func(ctx context.Context, w http.ResponseWriter, resp interface{}) (err error)

// NewUnaryHandler creates and returns an echo.HandlerFunc that accepts a single request
// and returns a single response
func NewUnaryHandler(e endpoint.Endpoint, dec decodeRequestFunc, enc encodeResponseFunc, errorEncoder errorEncoderFunc, l log.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()

		// Use logging response writer to try and catch where superfluous write header calls are coming from
		w := newLoggingResponseWriter(c.Request(), c.Response().Writer, l)

		req, err := dec(c, l)
		if err != nil {
			return errorEncoder(c, err)
		}

		resp, err := e(ctx, req)
		if err != nil {
			return errorEncoder(c, err)
		}

		if err := enc(ctx, w, resp); err != nil {
			return errorEncoder(c, err)
		}
		return nil
	}
}

type loggingResponseWriter struct {
	*sync.Mutex
	log log.Logger

	writer http.ResponseWriter
	req    *http.Request

	writeCounts       int
	writeHeaderCounts int
}

func newLoggingResponseWriter(r *http.Request, w http.ResponseWriter, l log.Logger) *loggingResponseWriter {
	return &loggingResponseWriter{
		Mutex:             &sync.Mutex{},
		log:               l,
		writer:            w,
		req:               r,
		writeCounts:       0,
		writeHeaderCounts: 0,
	}
}

func (l *loggingResponseWriter) Header() http.Header {
	return l.writer.Header()
}

func (l *loggingResponseWriter) Write(bytes []byte) (int, error) {
	l.Lock()
	defer func() {
		l.writeCounts += 1
		l.Unlock()
	}()

	if l.writeCounts > 0 {
		l.log.Error("more than one write header call", "url", l.req.URL.String(), "resp_body", fmt.Sprintf("%s", bytes), "write_counts", l.writeCounts)
	}

	return l.Write(bytes)
}

func (l *loggingResponseWriter) WriteHeader(statusCode int) {
	l.Lock()
	defer func() {
		l.writeHeaderCounts += 1
		l.Unlock()
	}()

	if l.writeHeaderCounts > 0 {
		l.log.Error("more than one write header call", "url", l.req.URL.String(), "status_code", statusCode)
	}

	l.writer.WriteHeader(statusCode)
}
