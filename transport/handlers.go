package transport

import (
	"context"
	"net/http"

	"github.com/go-kit/kit/endpoint"
	"github.com/harness/ff-proxy/domain"
	"github.com/labstack/echo/v4"
)

// errorEncoderFunc is a function for encoding errors and writing
// error responses
type errorEncoderFunc func(c echo.Context, err error) error

// decodeRequestFunc is a function that decodes http requests into a type
type decodeRequestFunc func(c echo.Context) (request interface{}, err error)

// encodeResponseFunc is a function for encoding http responses
type encodeResponseFunc func(ctx context.Context, w http.ResponseWriter, resp interface{}) (err error)

// NewUnaryHandler creates and returns an echo.HandlerFunc that accepts a single request
// and returns a single response
func NewUnaryHandler(e endpoint.Endpoint, dec decodeRequestFunc, enc encodeResponseFunc, errorEncoder errorEncoderFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		w := c.Response().Writer

		req, err := dec(c)
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

// NewServerStreamHandler creates and returns an echo.HandlerFunc that can return
// a stream of messages in response to a single response
func NewServerStreamHandler(e streamEndpoint, dec decodeRequestFunc, enc encodeResponseFunc, errorEncoder errorEncoderFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		w := c.Response().Writer

		req, err := dec(c)
		if err != nil {
			return errorEncoder(c, err)
		}

		stream := domain.NewStream(w)
		err = e(ctx, req, stream)
		if err != nil {
			return errorEncoder(c, err)
		}

		if err := enc(ctx, w, stream); err != nil {
			return errorEncoder(c, err)
		}
		return nil
	}
}
