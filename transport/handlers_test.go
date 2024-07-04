package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kit/kit/endpoint"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

type mockResponseWriter struct {
	rec              *httptest.ResponseRecorder
	headerCalls      int
	writeHeaderCalls int
	writeCalls       int
}

func (m *mockResponseWriter) Header() http.Header {
	m.headerCalls++
	return m.rec.Header()
}

func (m *mockResponseWriter) Write(bytes []byte) (int, error) {
	m.writeCalls++
	return m.rec.Write(bytes)
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.writeHeaderCalls++
	m.rec.WriteHeader(statusCode)
}

func Test_UnaryHandlerAuthRequest(t *testing.T) {

	type args struct {
		decodeFunc   decodeRequestFunc
		endpoint     endpoint.Endpoint
		encodeFunc   encodeResponseFunc
		errorEncoder errorEncoderFunc
		body         io.Reader
	}

	type mocks struct {
	}

	type expected struct {
		writeCalls       int
		writeHeaderCalls int
		headerCalls      int
	}

	testCases := map[string]struct {
		args      args
		mocks     mocks
		expected  expected
		shouldErr bool
	}{
		// Decode will return an error because we're sending a nil body
		"Given I have an auth request and the decode func returns an error": {
			args: args{
				decodeFunc: decodeAuthRequest,
				endpoint: func(ctx context.Context, request interface{}) (interface{}, error) {
					return nil, nil
				},
				encodeFunc:   encodeResponse,
				errorEncoder: encodeEchoError,
				body:         nil,
			},
			expected: expected{
				writeCalls:       1,
				writeHeaderCalls: 1,
				headerCalls:      2,
			},
			shouldErr: false,
		},
		"Given I have an auth request and the endpoint returns an error": {
			args: args{
				decodeFunc: decodeAuthRequest,
				endpoint: func(ctx context.Context, request interface{}) (interface{}, error) {
					return nil, errors.New("endpoint error")
				},
				encodeFunc:   encodeResponse,
				errorEncoder: encodeEchoError,
				body:         bytes.NewBuffer([]byte(`{"apiKey": "1234"}`)),
			},
			expected: expected{
				writeCalls:       1,
				writeHeaderCalls: 1,
				headerCalls:      2,
			},
			shouldErr: false,
		},
		"Given I have an auth request and the endpoint returns a valid response": {
			args: args{
				decodeFunc: decodeAuthRequest,
				endpoint: func(ctx context.Context, request interface{}) (interface{}, error) {
					return domain.AuthResponse{AuthToken: "1234"}, nil
				},
				encodeFunc:   encodeResponse,
				errorEncoder: encodeEchoError,
				body:         bytes.NewBuffer([]byte(`{"apiKey": "1234"}`)),
			},
			expected: expected{
				writeCalls:       1,
				writeHeaderCalls: 0, // Wonder why this is
				headerCalls:      1,
			},
			shouldErr: false,
		},
		"Given I have an auth request and the endpoint returns a valid response but the encodeResponseFunc errors": {
			args: args{
				decodeFunc: decodeAuthRequest,
				endpoint: func(ctx context.Context, request interface{}) (interface{}, error) {
					return domain.AuthResponse{AuthToken: "1234"}, nil
				},
				encodeFunc: func(ctx context.Context, w http.ResponseWriter, resp interface{}) (err error) {
					return errors.New("encode error")
				},
				errorEncoder: encodeEchoError,
				body:         bytes.NewBuffer([]byte(`{"apiKey": "1234"}`)),
			},
			expected: expected{
				writeCalls:       1,
				writeHeaderCalls: 1, // Wonder why this is
				headerCalls:      2,
			},
			shouldErr: false,
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			e := echo.New()

			l, _ := log.NewStructuredLogger("INFO")

			handler := NewUnaryHandler(tc.args.endpoint, tc.args.decodeFunc, tc.args.encodeFunc, tc.args.errorEncoder, l)

			req := httptest.NewRequest(http.MethodPost, "/", tc.args.body)
			rec := &mockResponseWriter{rec: httptest.NewRecorder()}

			c := e.NewContext(req, rec)

			err := handler(c)
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.writeCalls, rec.writeCalls)
			assert.Equal(t, tc.expected.writeHeaderCalls, rec.writeHeaderCalls)
			assert.Equal(t, tc.expected.headerCalls, rec.headerCalls)
		})
	}
}
