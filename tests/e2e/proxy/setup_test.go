package proxy

import (
	"flag"
	"fmt"
	"github.com/avast/retry-go"
	"github.com/harness/ff-golang-server-sdk/log"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	initLog()
	var env string
	// default to .env.qa if none specified
	flag.StringVar(&env, "env", ".env.qa", "env file name")
	flag.Parse()

	err := godotenv.Load(fmt.Sprintf("../env/tests/%s", env))
	if err != nil {
		log.Fatal(err)
	}

	err = setup()
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	exitVal := m.Run()
	teardown()
	os.Exit(exitVal)
}

func initLog() {
	config := zap.NewDevelopmentConfig()

	config.EncoderConfig.LevelKey = "severity"
	config.EncoderConfig.MessageKey = "message"
	config.Level.SetLevel(zapcore.DebugLevel)

	logger, err := config.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	log.SetLogger(logger.Sugar())
}

func setup() error {
	err := retry.Do(
		func() error {
			resp, err := http.Get(fmt.Sprintf("http://localhost:7000/health"))
			if err != nil {
				log.Warn(err.Error())
				return err
			}

			if resp.StatusCode == http.StatusOK {
				log.Infof("heartbeat healthy: status code: %d", resp.StatusCode)
				return nil
			}

			b, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Warn(fmt.Sprintf("failed to read response body from %s", resp.Request.URL.String()))
				return fmt.Errorf("failed to read response body from %s", resp.Request.URL.String())
			}

			log.Warn(fmt.Sprintf("heartbeat unhealthy: status code: %d, body: %s", resp.StatusCode, b))
			return fmt.Errorf(fmt.Sprintf("heartbeat unhealthy: status code: %d, body: %s", resp.StatusCode, b))
		},
		retry.Attempts(20), retry.Delay(5*time.Second),
	)

	if err != nil {
		return err
	}

	return nil
}

func teardown() {
	log.Info("Shutting down tests")
}

// GetClientURL ...
func GetClientURL() string {
	return os.Getenv("CLIENT_URL")
}

// GetEventsURL ...
func GetEventsURL() string {
	return os.Getenv("EVENTS_URL")
}

// GetAPIKey ...
func GetAPIKey() string {
	return os.Getenv("API_KEY")
}
