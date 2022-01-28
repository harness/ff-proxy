package proxy

import (
	"crypto/tls"
	harness "github.com/harness/ff-golang-server-sdk/client"
	"github.com/harness/ff-golang-server-sdk/evaluation"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
)

func Test_ServerSDK(t *testing.T) {
	var client *harness.CfClient
	defaultTarget := evaluation.Target{
		Identifier: "test",
		Name:       "test",
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	sdkClient := &http.Client{Transport: tr}

	var err error
	client, err = harness.NewCfClient(GetAPIKey(),
		harness.WithURL(GetClientURL()),
		harness.WithHTTPClient(sdkClient),
		harness.WithEventsURL(GetEventsURL()),
		harness.WithStreamEnabled(false),
		harness.WithStoreEnabled(false),
	)
	if err != nil {
		t.Error(err)
	}

	ok, err := client.IsInitialized()
	if !ok {
		t.Error("Test SDK failed to start")
	}
	if err != nil {
		t.Error(err)
	}

	t.Cleanup(func() {
		client.Close()
	})

	t.Run("testBoolFlag", func(t *testing.T) {
		flag, _ := client.BoolVariation("test", &defaultTarget, false)
		assert.True(t, flag)
	})
}
