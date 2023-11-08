package e2e

import (
	"testing"

	harness "github.com/harness/ff-golang-server-sdk/client"
	"github.com/harness/ff-golang-server-sdk/evaluation"
	"github.com/stretchr/testify/assert"

	"github.com/harness/ff-proxy/v2/tests/e2e/testhelpers"
)

var emptyTarget = evaluation.Target{}

func TestServerSDK(t *testing.T) {

	type args struct {
		Target evaluation.Target
		Flag   string
	}
	tests := map[string]struct {
		args args
		want interface{}
	}{
		"Test basic bool": {
			args{
				Target: emptyTarget,
				Flag:   "bool-flag1",
			},
			true,
		},
		"Test basic string": {
			args{
				Target: emptyTarget,
				Flag:   "string-flag1",
			},
			"red",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			svrKey := GetServerAPIKey()
			url := GetStreamURL()
			t.Logf("key %s url %s", svrKey, url)

			client, err := harness.NewCfClient(GetServerAPIKey(),
				harness.WithURL(GetStreamURL()),
				harness.WithEventsURL(GetStreamURL()),
				harness.WithStreamEnabled(false),
				harness.WithTarget(tt.args.Target),
				harness.WithHTTPClient(testhelpers.GetCertClient()),
			)
			if err != nil {
				t.Fatalf("Couldn't create sdk err %s", err)
			}
			init, err := client.IsInitialized()
			if !init || err != nil {
				t.Fatalf("SDK didn't initialise err %s", err)
			}

			var actual interface{}
			switch tt.want.(type) {
			case bool:
				actual, err = client.BoolVariation(tt.args.Flag, &tt.args.Target, false)
			case string:
				actual, err = client.StringVariation(tt.args.Flag, &tt.args.Target, "")
			case int64:
				actual, err = client.IntVariation(tt.args.Flag, &tt.args.Target, 0)
			case float64:
				actual, err = client.NumberVariation(tt.args.Flag, &tt.args.Target, 0.0)
			default:
				t.Fatalf("want type didn't match any supported types want: %s", tt.want)
			}

			if err != nil {
				t.Fatalf("couldn't evaluate flag %s", tt.args.Flag)
			}
			assert.Equal(t, tt.want, actual)

		})
	}
}
