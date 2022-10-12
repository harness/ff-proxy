package e2e

import (
	"flag"
	"fmt"
	"os"
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

// GetStreamURL ...
func GetStreamURL() string {
	return os.Getenv("STREAM_URL")
}
