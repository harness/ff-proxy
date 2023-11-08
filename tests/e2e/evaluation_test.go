package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/harness/ff-proxy/v2/gen/client"
	"github.com/harness/ff-proxy/v2/tests/e2e/testhelpers"
)

var expectedEvaluations = map[string]client.Evaluation{
	"string-flag1": {
		Flag:       "string-flag1",
		Identifier: strToPtr("red"),
		Kind:       "string",
		Value:      "red",
	},
	"bool-flag1": {
		Flag:       "bool-flag1",
		Identifier: strToPtr("true"),
		Kind:       "boolean",
		Value:      "true",
	},
}

func strToPtr(str string) *string {
	return &str
}

// test /client/env/:environment_uuid/target/:target/evaluations/:feature endpoint
func TestEvaluationsByFeature(t *testing.T) {
	clientTarget := client.Target{
		Identifier: "target",
		Name:       "target",
	}
	token, claims, err := testhelpers.AuthenticateSDKClient(GetServerAPIKey(), GetStreamURL(), &clientTarget)
	if err != nil {
		t.Error(err)
	}
	envID := claims.Environment

	type args struct {
		FlagName   string
		TargetName string
	}
	type result struct {
		StatusCode int
		Value      string
	}
	tests := map[string]struct {
		args    args
		want    result
		wantErr bool
	}{
		"Bool gives correct result with valid target": {
			args: args{
				FlagName:   "bool-flag1",
				TargetName: clientTarget.Identifier,
			},
			want: result{
				StatusCode: 200,
				Value:      "true",
			},
			wantErr: false,
		},
		"String gives correct result with valid target": {
			args: args{
				FlagName:   "string-flag1",
				TargetName: clientTarget.Identifier,
			},
			want: result{
				StatusCode: 200,
				Value:      "red",
			},
			wantErr: false,
		},
		"Target that doesnt exist returns results": {
			args: args{
				FlagName:   "string-flag1",
				TargetName: "doesntexist",
			},
			want: result{
				StatusCode: 200,
				Value:      "red",
			},
			wantErr: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			resp, err := evaluateFlag(envID, tt.args.TargetName, tt.args.FlagName, token)

			assert.NoError(t, err)
			assert.Equal(t, tt.want.StatusCode, resp.StatusCode())
			if !tt.wantErr {
				assert.Equal(t, tt.want.Value, resp.JSON200.Value)
			}
		})
	}
}

// test /client/env/:environment_uuid/target/:target/evaluations endpoint
func TestEvaluations(t *testing.T) {
	clientTarget := client.Target{
		Identifier: "target",
		Name:       "target",
	}

	type args struct {
		APIKey     string
		TargetName string
	}
	type result struct {
		StatusCode int
		Results    map[string]client.Evaluation
	}
	tests := map[string]struct {
		args    args
		want    result
		wantErr bool
	}{
		"Valid target gets correct results": {
			args: args{
				APIKey:     GetServerAPIKey(),
				TargetName: clientTarget.Identifier,
			},
			want: result{
				StatusCode: 200,
				Results:    expectedEvaluations,
			},
			wantErr: false,
		},
		"Target that doesnt exist returns results": {
			args: args{
				APIKey:     GetServerAPIKey(),
				TargetName: "doesntexist",
			},
			want: result{
				StatusCode: 200,
				Results:    expectedEvaluations,
			},
			wantErr: true,
		},
		"Valid target gets empty results for empty project": {
			args: args{
				APIKey:     GetEmptyProjectServerAPIKey(),
				TargetName: clientTarget.Identifier,
			},
			want: result{
				StatusCode: 200,
				Results:    map[string]client.Evaluation{},
			},
			wantErr: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if tt.args.APIKey == "" {
				t.Skipf("api key not provided for test %s, skipping", t.Name())
			}
			token, claims, err := testhelpers.AuthenticateSDKClient(tt.args.APIKey, GetStreamURL(), &clientTarget)
			if err != nil {
				t.Error(err)
			}
			envID := claims.Environment
			resp, err := evaluateFlags(envID, tt.args.TargetName, token)

			assert.NoError(t, err)
			assert.Equal(t, tt.want.StatusCode, resp.StatusCode)
			if !tt.wantErr {
				// marshal response
				evals, err := parseEvaluationResp(resp)
				if err != nil {
					t.Error("couldn't parse client response")
				}

				assert.Equal(t, len(tt.want.Results), len(evals))
				for _, eval := range evals {
					expected := expectedEvaluations[eval.Flag]
					assert.Equal(t, expected.Flag, eval.Flag)
					assert.Equal(t, expected.Value, eval.Value)
					assert.Equal(t, expected.Identifier, eval.Identifier)
					assert.Equal(t, expected.Kind, eval.Kind)
				}
			}
		})
	}
}

/*
*
Helper functions and structs to support the tests above
*/
func evaluateFlag(envID, target, feature, token string) (*client.GetEvaluationByIdentifierResponse, error) {
	c := testhelpers.DefaultEvaluationClient(GetStreamURL())
	resp, err := c.GetEvaluationByIdentifier(context.Background(), envID, target, feature, &client.GetEvaluationByIdentifierParams{}, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("authorization", fmt.Sprintf("Bearer %s", token))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return client.ParseGetEvaluationByIdentifierResponse(resp)
}

func evaluateFlags(envID, target, token string) (*http.Response, error) {
	c := testhelpers.DefaultEvaluationClient(GetStreamURL())
	resp, err := c.GetEvaluations(context.Background(), envID, target, &client.GetEvaluationsParams{}, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("authorization", fmt.Sprintf("Bearer %s", token))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func parseEvaluationResp(resp *http.Response) ([]client.Evaluation, error) {
	// marshal response
	var dest []client.Evaluation
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	defer func() { _ = resp.Body.Close() }()
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(bodyBytes, &dest); err != nil {
		return nil, err
	}
	return dest, nil
}
