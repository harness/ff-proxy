package transport

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	jsoniter "github.com/json-iterator/go"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func Test_isIdentifierValid(t *testing.T) {
	type args struct {
		identifier string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"Alphanumeric is valid",
			args{
				identifier: "TargetName123",
			},
			true,
		},
		{
			"Spaces are invalid",
			args{
				identifier: "target name",
			},
			false,
		},
		{
			"Special characters are invalid",
			args{
				identifier: "target$({}><?/",
			},
			false,
		},
		{
			"Emails are valid",
			args{
				identifier: "test@harness.io",
			},
			true,
		},
		{
			"Underscore is valid",
			args{
				identifier: "__global__cf_target",
			},
			true,
		},
		{
			"Dash is valid",
			args{
				identifier: "test-user",
			},
			true,
		},
		{
			"Single character is valid",
			args{
				identifier: "t",
			},
			true,
		},
		{
			"Empty string is invalid",
			args{
				identifier: "",
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIdentifierValid(tt.args.identifier); got != tt.want {
				t.Errorf("IsIdentifierValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isNameValid(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"Alphanumeric is valid",
			args{
				name: "TargetName123",
			},
			true,
		},
		{
			"Spaces are valid",
			args{
				name: "Global Target",
			},
			true,
		},
		{
			"Special characters are invalid",
			args{
				name: "target$({}><?/",
			},
			false,
		},
		{
			"Emails are valid",
			args{
				name: "test@harness.io",
			},
			true,
		},
		{
			"Underscore is valid",
			args{
				name: "__global__cf_target",
			},
			true,
		},
		{
			"Dash is valid",
			args{
				name: "test-user",
			},
			true,
		},
		{
			"Empty string is invalid",
			args{
				name: "",
			},
			false,
		},
		{
			"Unicode characters are valid",
			args{
				name: "ńoooo",
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNameValid(tt.args.name); got != tt.want {
				t.Errorf("IsIdentifierValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_decodeGetEvaluationsRequest(t *testing.T) {
	target := domain.Target{
		Target: clientgen.Target{
			Attributes: domain.ToPtr(map[string]interface{}{
				"email": "foo@gmail.com",
			}),
			Identifier: "foo",
			Name:       "bar",
		},
	}

	b, err := jsoniter.Marshal(target)
	assert.Nil(t, err)

	encodedTarget := base64.StdEncoding.EncodeToString(b)

	type args struct {
		envID   string
		target  string
		headers map[string]string
	}

	type expected struct {
		req domain.EvaluationsRequest
	}

	testCases := map[string]struct {
		args      args
		expected  expected
		shouldErr bool
	}{
		"Given I make a evaluations request and include a target header": {
			args: args{
				envID:   "123",
				target:  "foo",
				headers: map[string]string{targetHeader: encodedTarget},
			},
			shouldErr: false,
			expected: expected{
				req: domain.EvaluationsRequest{
					EnvironmentID:    "123",
					TargetIdentifier: "foo",
					Target:           &target,
				},
			},
		},
		"Given I make a evaluations request and don't include a target header": {
			args: args{
				envID:   "123",
				target:  "foo",
				headers: nil,
			},
			shouldErr: false,
			expected: expected{
				req: domain.EvaluationsRequest{
					EnvironmentID:    "123",
					TargetIdentifier: "foo",
					Target:           nil,
				},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for h, v := range tc.args.headers {
				req.Header.Set(h, v)
			}

			e := echo.New()
			c := e.NewContext(req, httptest.NewRecorder())
			c.SetPath(evaluationsFlagRoute)
			c.SetParamNames("environment_uuid", "target")
			c.SetParamValues(tc.args.envID, tc.args.target)

			actual, err := decodeGetEvaluationsRequest(c)
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.req, actual)
		})
	}
}

func Test_decodeGetEvaluationsByFeatureRequest(t *testing.T) {
	target := domain.Target{
		Target: clientgen.Target{
			Attributes: domain.ToPtr(map[string]interface{}{
				"email": "foo@gmail.com",
			}),
			Identifier: "foo",
			Name:       "bar",
		},
	}

	b, err := jsoniter.Marshal(target)
	assert.Nil(t, err)

	encodedTarget := base64.StdEncoding.EncodeToString(b)

	type args struct {
		envID   string
		target  string
		feature string
		headers map[string]string
	}

	type expected struct {
		req domain.EvaluationsByFeatureRequest
	}

	testCases := map[string]struct {
		args      args
		expected  expected
		shouldErr bool
	}{
		"Given I make a evaluations request and include a target header": {
			args: args{
				envID:   "123",
				target:  "foo",
				feature: "booleanFlag",
				headers: map[string]string{targetHeader: encodedTarget},
			},
			shouldErr: false,
			expected: expected{
				req: domain.EvaluationsByFeatureRequest{
					EnvironmentID:     "123",
					TargetIdentifier:  "foo",
					FeatureIdentifier: "booleanFlag",
					Target:            &target,
				},
			},
		},
		"Given I make a evaluations request and don't include a target header": {
			args: args{
				envID:   "123",
				target:  "foo",
				feature: "booleanFlag",
				headers: nil,
			},
			shouldErr: false,
			expected: expected{
				req: domain.EvaluationsByFeatureRequest{
					EnvironmentID:     "123",
					TargetIdentifier:  "foo",
					FeatureIdentifier: "booleanFlag",
					Target:            nil,
				},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for h, v := range tc.args.headers {
				req.Header.Set(h, v)
			}

			e := echo.New()
			c := e.NewContext(req, httptest.NewRecorder())
			c.SetPath(evaluationsFlagRoute)
			c.SetParamNames("environment_uuid", "target", "feature")
			c.SetParamValues(tc.args.envID, tc.args.target, tc.args.feature)

			actual, err := decodeGetEvaluationsByFeatureRequest(c)
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			assert.Equal(t, tc.expected.req, actual)
		})
	}
}
