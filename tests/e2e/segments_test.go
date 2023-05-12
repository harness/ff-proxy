package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/harness/ff-proxy/gen/client"
	"github.com/harness/ff-proxy/tests/e2e/testhelpers"

	"github.com/stretchr/testify/assert"
)

var (
	// subset of expected segment data
	expectedSegments = map[string]client.Segment{"group": {
		Environment: strToPtr("default"),
		Excluded:    nil,
		Identifier:  "group",
		Included:    nil,
		Name:        "groupName",
		Rules:       nil,
		Version:     nil,
	}}
)

func TestTargetSegments(t *testing.T) {

	type args struct {
		APIKey string
		EnvID  string
	}
	type result struct {
		StatusCode int
		Segments   map[string]client.Segment
	}
	tests := map[string]struct {
		args    args
		want    result
		wantErr bool
	}{
		"Test GetFeatureSegments succeeds for valid request": {
			args: args{
				APIKey: GetServerAPIKey(),
			},
			want: result{
				StatusCode: 200,
				Segments:   expectedSegments,
			},
			wantErr: false,
		},
		"Test GetFeatureSegments succeeds for empty project": {
			args: args{
				APIKey: GetEmptyProjectServerAPIKey(),
			},
			want: result{
				StatusCode: 200,
				Segments:   map[string]client.Segment{},
			},
			wantErr: false,
		},
		// TODO - this comes back with empty values right now, we should match ff-server behaviour
		//"Test GetFeatureSegments for invalid env uuid": {
		//	args: args{
		//		APIKey: GetEmptyProjectServerAPIKey(),
		//		EnvID:  "invalid",
		//	},
		//	want: result{
		//		StatusCode: 400,
		//	},
		//	wantErr: true,
		//},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// auth to get token
			if tt.args.APIKey == "" {
				t.Skipf("api key not provided for test %s, skipping", t.Name())
			}
			token, claims, err := testhelpers.AuthenticateSDKClient(tt.args.APIKey, GetStreamURL(), nil)
			if err != nil {
				t.Error(err)
			}
			envID := claims.Environment
			// set custom env if provided
			if tt.args.EnvID != "" {
				envID = tt.args.EnvID
			}

			got, err := GetAllSegments(t, envID, token)
			if err != nil {
				t.Error(err)
			}
			// Assert the response
			assert.NotNil(t, got)
			assert.Equal(t, tt.want.StatusCode, got.StatusCode(), "expected http status code %d but got %d", tt.want.StatusCode, got.StatusCode())

			if got.StatusCode() == 200 {
				assert.False(t, tt.wantErr)
				assert.NotNil(t, got.JSON200)
				assert.Equal(t, len(tt.want.Segments), len(*got.JSON200))

				for _, segment := range *got.JSON200 {
					expectedSegment, ok := expectedSegments[segment.Identifier]
					assert.True(t, ok)
					assert.Equal(t, *expectedSegment.Environment, *segment.Environment)
					assert.Equal(t, expectedSegment.Identifier, segment.Identifier)
					assert.Equal(t, expectedSegment.Name, segment.Name)
				}
			} else {
				assert.True(t, tt.wantErr)
			}
		})
	}
}

func GetAllSegments(t *testing.T, envID string, token string) (*client.GetAllSegmentsResponse, error) {
	cfClient := testhelpers.DefaultEvaluationClient(GetStreamURL())

	resp, err := cfClient.GetAllSegments(context.Background(), envID, &client.GetAllSegmentsParams{}, func(ctx context.Context, req *http.Request) error {
		req.Header.Set("authorization", fmt.Sprintf("Bearer %s", token))
		return nil
	})
	if err != nil {
		t.Errorf("get all segment request failed with reason %s", err)
	}
	return client.ParseGetAllSegmentsResponse(resp)
}
