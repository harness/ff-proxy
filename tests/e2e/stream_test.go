package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	harness "github.com/harness/ff-golang-server-sdk/client"
	"github.com/harness/ff-golang-server-sdk/stream"
	"github.com/stretchr/testify/assert"

	"github.com/harness/ff-proxy/v2/gen/admin"
	"github.com/harness/ff-proxy/v2/tests/e2e/testhelpers"
)

// TestEvent tests GET /client/stream event emissions
func TestEvent(t *testing.T) {
	if !IsOnline() {
		t.Skipf("Skipping streaming tests. Running in offline mode")
	}

	type args struct {
		Key       string
		Operation func() error
	}
	type expectedResults struct {
		sseEvent  stream.Message
		flagValue string
	}
	tests := map[string]struct {
		args    args
		want    expectedResults
		wantErr bool
	}{
		"Test Patch Flag SSE Event Sent When string-flag1 enabled": {
			args{
				Key: GetServerAPIKey(),
				Operation: func() error {
					parameters := make(map[string]interface{})
					parameters["state"] = "on"
					resp := PatchFeatureFlag(t, DefaultClient(), GetAccountIdentifier(), GetOrgIdentifier(), "string-flag1", GetProjectIdentifier(), GetEnvironmentIdentifier(), "setFeatureFlagState", parameters)
					if resp.StatusCode != 200 {
						return fmt.Errorf("non 200 status code, got %d", resp.StatusCode)
					}
					return nil
				},
			},
			expectedResults{stream.Message{
				Event:      "patch",
				Domain:     "flag",
				Identifier: "string-flag1",
			}, "blue"},
			false,
		},
		"Test Patch Flag SSE Event Sent When string-flag1 disabled": {
			args{
				Key: GetServerAPIKey(),
				Operation: func() error {
					parameters := make(map[string]interface{})
					parameters["state"] = "off"
					resp := PatchFeatureFlag(t, DefaultClient(), GetAccountIdentifier(), GetOrgIdentifier(), "string-flag1", GetProjectIdentifier(), GetEnvironmentIdentifier(), "setFeatureFlagState", parameters)
					if resp.StatusCode != 200 {
						return fmt.Errorf("non 200 status code, got %d", resp.StatusCode)
					}
					return nil
				},
			},
			expectedResults{stream.Message{
				Event:      "patch",
				Domain:     "flag",
				Identifier: "string-flag1",
			}, "red"},
			false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			eventChan := make(chan stream.Message, 10)

			eventListener := EventListener{eventChan}
			// if running locally with https enabled using the self-signed certs in /certs folder you need to add the cert
			// to your trusted list like we do in the e2e test pipeline - this is because
			// the go sdk sse client creates it's own http client which doesn't use the trusted certs we load in GetCertClient
			client, err := harness.NewCfClient(tt.args.Key,
				harness.WithURL(GetStreamURL()),
				harness.WithEventStreamListener(eventListener),
				harness.WithHTTPClient(testhelpers.GetCertClient()),
				harness.WithProxyMode(true),
			)
			if err != nil {
				t.Error("Couldn't create sdk")
			}
			init, err := client.IsInitialized()
			if !init || err != nil {
				t.Error("SDK didn't initialise")
			}

			// run whatever crud operation we want here to trigger an event of some sort
			err = tt.args.Operation()
			if err != nil {
				t.Errorf("Failed to perform operation: %s", err)
			}

			// wait for up to 10 seconds for the expected sse event to come in
			select {
			case msg := <-eventChan:
				assert.Equal(t, tt.want.sseEvent.Event, msg.Event)
				assert.Equal(t, tt.want.sseEvent.Domain, msg.Domain)
				assert.Equal(t, tt.want.sseEvent.Identifier, msg.Identifier)
			case <-time.After(10 * time.Second):
				t.Error("Timed out waiting for event to come in")
			}
			result, err := client.StringVariation("string-flag1", nil, "default")
			assert.Nil(t, err)
			assert.Equal(t, tt.want.flagValue, result)
		})
	}
}

// EventListener implements the golang sdks stream.EventStreamListener interface
// and can be used to hook into the SDK to receive SSE Events that are sent to
// it by the FeatureFlag server.
type EventListener struct {
	eventChan chan stream.Message
}

// Pub makes EventListener implement the golang sdks stream.EventStreamListener
// interface.
func (e EventListener) Pub(ctx context.Context, event stream.Event) error {
	if event.SSEEvent == nil {
		return errors.New("received nil SSE event")
	}

	// read event message
	msg := stream.Message{}
	err := json.Unmarshal(event.SSEEvent.Data, &msg)
	if err != nil {
		return err
	}

	e.eventChan <- msg
	return nil
}

// PatchFeatureFlag ...
func PatchFeatureFlag(t *testing.T, client *admin.Client, account string, org string, flagIdentifier string,
	projectIdentifier string, envIdentifier string, kind string, parameters map[string]interface{}) *http.Response {
	eoq := envIdentifier
	feature, err := client.PatchFeature(context.Background(), admin.Identifier(flagIdentifier), &admin.PatchFeatureParams{
		AccountIdentifier:     admin.AccountQueryParam(account),
		OrgIdentifier:         admin.OrgQueryParam(org),
		ProjectIdentifier:     admin.ProjectQueryParam(projectIdentifier),
		EnvironmentIdentifier: (*admin.EnvironmentOptionalQueryParam)(&eoq),
	}, admin.PatchFeatureJSONRequestBody{
		Comment:       nil,
		ExecutionTime: nil,
		Instructions: admin.PatchInstruction{
			{
				Kind:       kind,
				Parameters: parameters,
			},
		},
	}, AddAuthToken)

	if err != nil {
		t.Error(err)
	}
	return feature
}

// AddAuthToken adds the user access token to the request
func AddAuthToken(ctx context.Context, req *http.Request) error {
	if GetUserAccessToken() != "" {
		req.Header.Set("x-api-key", GetUserAccessToken())
	}
	return nil
}

// DefaultClient returns the default admin client
func DefaultClient() *admin.Client {
	client, err := admin.NewClient(GetRemoteURL())
	client.Client = testhelpers.GetCertClient()

	if err != nil {
		return nil
	}
	return client
}
