package e2e

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	Setup()
	exitVal := m.Run()
	Teardown()
	os.Exit(exitVal)
}

// TestSetup performs setup before a test.  This includes initalizing logging, and working
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

// GetProjectIdentifier ...
func GetProjectIdentifier() string {
	return os.Getenv("PROJECT_IDENTIFIER")
}

// GetEnvironmentIdentifier ...
func GetEnvironmentIdentifier() string {
	return os.Getenv("ENVIRONMENT_IDENTIFIER")
}
