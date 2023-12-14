package e2e

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/harness/ff-proxy/v2/tests/e2e/testhelpers"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	Setup()
	exitVal := m.Run()
	Teardown()
	os.Exit(exitVal)
}

// TestSetup performs setup before a test.  This includes initializing logging, and working
// out how to authenticate with the platform
func Setup() {
	log.Infof("Global Test Setup")
	var env string
	// default to .env.local file if none specified
	flag.StringVar(&env, "env", ".env.offline", "env file name")
	flag.Parse()
	log.Debug(env)
	err := godotenv.Load(fmt.Sprintf("env/%s", env))
	if err != nil {
		log.Fatal(err)
	}

	for _, x := range os.Environ() {
		log.Infof("%s", x)
	}

	testhelpers.SetAuthToken(testhelpers.GetUserAccessToken())
	// wait for service to be healthy

}

// TestTeardown performs cleanup following a test run
func Teardown() {
	log.Infof("Global Test Teardown")
}

// GetServerAPIKey ...
func GetServerAPIKey() string {
	return os.Getenv("SERVER_API_KEY")
}

// GetEmptyProjectServerAPIKey ...
func GetEmptyProjectServerAPIKey() string {
	return os.Getenv("EMPTY_PROJECT_API_KEY")
}

// GetStreamURL ...
func GetStreamURL() string {
	return os.Getenv("STREAM_URL")
}

// GetUserAccessToken ...
func GetUserAccessToken() string {
	return os.Getenv("USER_ACCESS_TOKEN")
}

// IsOnline ...
func IsOnline() bool {
	online, err := strconv.ParseBool(os.Getenv("ONLINE"))
	if err != nil {
		log.Warn("Couldn't parse ONLINE environment variable as bool")
	}
	return online
}

// RunMetricsTests ...
func RunMetricsTests() bool {
	runMetricsTests, err := strconv.ParseBool(os.Getenv("RUN_METRICS_TESTS"))
	if err != nil {
		log.Warn("Couldn't parse RUN_METRICS_TESTS environment variable as bool")
	}
	return runMetricsTests
}

// GetRemoteURL ...
func GetRemoteURL() string {
	return os.Getenv("REMOTE_URL")
}

// GetAccountIdentifier ...
func GetAccountIdentifier() string {
	return os.Getenv("ACCOUNT_IDENTIFIER")
}

// GetOrgIdentifier ...
func GetOrgIdentifier() string {
	return os.Getenv("ORG_IDENTIFIER")
}

// GetOrgIdentifier ...
func GetSecondaryOrgIdentifier() string {
	return os.Getenv("SECONDARY_ORG")
}

// GetProjectIdentifier ...
func GetProjectIdentifier() string {
	return os.Getenv("PROJECT_IDENTIFIER")
}

// GetSecondaryProjectIdentifier ...
func GetSecondaryProjectIdentifier() string {
	return os.Getenv("SECONDARY_PROJECT_IDENTIFIER")
}

// GetThirdProjectIdentifier ...
func GetThirdProjectIdentifier() string {
	return os.Getenv("THIRD_PROJECT_IDENTIFIER")
}

// GetThirdProjectIdentifier ...
func GetFourthProjectIdentifier() string {
	return os.Getenv("FOURTH_PROJECT_IDENTIFIER")
}

// GetEnvironmentIdentifier ...
func GetEnvironmentIdentifier() string {
	return os.Getenv("ENVIRONMENT_IDENTIFIER")
}

func GetDefaultEnvironmentID() string {
	return os.Getenv("DEFAULT_ENVIRONMENT_ID")
}
