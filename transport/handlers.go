package transport

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"syscall"

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
//type encodeResponseFunc func(ctx context.Context, w http.ResponseWriter, resp interface{}) (err error)

// encodeResponseFunc is a function for encoding http responses
type encodeResponseFunc func(c echo.Context, resp interface{}) (err error)

// NewUnaryHandler creates and returns an echo.HandlerFunc that accepts a single request
// and returns a single response
func NewUnaryHandler(e endpoint.Endpoint, dec decodeRequestFunc, enc encodeResponseFunc, errorEncoder errorEncoderFunc, l log.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()

		bl := NewBufferedLogger(l, defaultBufferSize)

		// Use logging response writer to try and catch where superfluous write header calls are coming from
		c.Response().Writer = newLoggingResponseWriter(c.Request(), c.Response().Writer, bl)

		req, err := dec(c, l)
		if err != nil {
			return errorEncoder(c, err)
		}

		resp, err := e(ctx, req)
		if err != nil {
			return errorEncoder(c, err)
		}

		if err := enc(c, resp); err != nil {
			return errorEncoder(c, err)
		}
		//if err := enc(ctx, w, resp); err != nil {
		//	return errorEncoder(c, err)
		//}
		return nil
	}
}

var matchStr = regexp.MustCompile(`/client/env/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/target/[^/]+/evaluations\?cluster=\d+`)

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
	defer l.Unlock()
	defer func() {
		l.writeCounts += 1
	}()

	if ok := matchStr.MatchString(l.req.URL.String()); ok {
		l.log.Info("first write call for evaluations request", "url", l.req.URL.String(), "resp_body", string(bytes), "write_counts", l.writeCounts)
	}

	if l.writeCounts > 0 {
		l.log.Error("more than one write call", "url", l.req.URL.String(), "resp_body", fmt.Sprintf("%s", bytes), "write_counts", l.writeCounts)
	}

	n, err := l.writer.Write(bytes)
	if err != nil {
		if errors.Is(err, syscall.EPIPE) {
			l.log.Error("failed to write response because client disconnected", "url", l.req.URL.String(), "resp_body", fmt.Sprintf("%s", bytes))
		}
		return n, err
	}

	return n, nil
}

func (l *loggingResponseWriter) WriteHeader(statusCode int) {
	l.Lock()
	defer l.Unlock()
	defer func() {
		l.writeHeaderCounts += 1
	}()

	if l.writeHeaderCounts > 0 {
		l.log.Error("more than one write header call", "url", l.req.URL.String(), "status_code", statusCode)
	}

	l.writer.WriteHeader(statusCode)
}

const defaultBufferSize = 16384

// BufferedLogger represents a logger that buffers messages and flushes on errors or buffer full.
type BufferedLogger struct {
	*sync.Mutex
	log.Logger
	buffer  *bytes.Buffer
	maxSize int
}

// NewBufferedLogger creates a new BufferedLogger.
func NewBufferedLogger(l log.Logger, maxSize int) *BufferedLogger {
	if maxSize <= 0 {
		maxSize = defaultBufferSize
	}

	return &BufferedLogger{
		Mutex:   &sync.Mutex{},
		Logger:  l,
		buffer:  &bytes.Buffer{},
		maxSize: maxSize,
	}
}

// Info buffers a log message and flushes if an error is logged or buffer is full.
func (bl *BufferedLogger) Info(message string, keyvals ...interface{}) {
	bl.Lock()
	defer bl.Unlock()

	foo := make([]string, len(keyvals))

	for _, key := range keyvals {
		keys, ok := key.(string)
		if ok {
			foo = append(foo, fmt.Sprintf("%s", keys))
		}
	}

	// Buffer the log message
	bl.buffer.WriteString(message + "keys:" + strings.Join(foo, "") + "\n")

	// Flush if the buffer is full
	if bl.buffer.Len() >= bl.maxSize {
		bl.flushBuffer()
	}
}

// Error buffers a log message and flushes if an error is logged or buffer is full.
func (bl *BufferedLogger) Error(message string, keyvals ...interface{}) {
	bl.Lock()
	defer bl.Unlock()

	// Flush the buffer first
	bl.flushBuffer()
	// Log the error immediately
	bl.Logger.Error(message, keyvals...)
}

// flushBuffer flushes the buffered log messages to the underlying logger.
func (bl *BufferedLogger) flushBuffer() {
	if bl.buffer.Len() > 0 {
		bl.Logger.Error("flushing buffered logger", "msg", bl.buffer.String())
		bl.buffer.Reset()
	}
}
