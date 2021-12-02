package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	harness "github.com/harness/ff-golang-server-sdk/client"
	"github.com/harness/ff-golang-server-sdk/logger"

	"github.com/harness/ff-golang-server-sdk/dto"
)

var (
	apiKey           string = ""
	featureFlag      string = ""
	targetIdentifier string = ""
	baseURL          string = ""
	eventsURL        string = ""
	streaming        bool   = false
)

func init() {
	flag.StringVar(&apiKey, "api-key", "c25e3f4e-9d2d-42d6-a85c-6fb3af062732", "api key to use")
	flag.StringVar(&featureFlag, "feature-flag", "harnessappdemodarkmode", "the feature flag to use, if left empty defaults are used")
	flag.StringVar(&targetIdentifier, "target-identifier", "james", "the identifier of the target you want the SDK to use")
	flag.StringVar(&baseURL, "baseURL", "http://localhost:7000", "The base url to use")
	flag.StringVar(&eventsURL, "eventsURL", "http://localhost:7000", "The events url to use")
	flag.BoolVar(&streaming, "streaming", false, "whether streaming is enabled")
	flag.Parse()
}

func main() {
	logger, err := logger.NewZapLogger(true)
	if err != nil {
		log.Fatal(err)
	}

	target := dto.NewTargetBuilder(targetIdentifier).
		Name(targetIdentifier).
		Build()

	client, err := harness.NewCfClient(apiKey,
		harness.WithStreamEnabled(streaming),
		harness.WithURL(baseURL),
		harness.WithEventsURL(eventsURL),
		harness.WithTarget(target),
		harness.WithLogger(logger),
		harness.WithPullInterval(1),
	)
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("error while closing client err: %v", err)
		}
	}()

	if err != nil {
		log.Printf("could not connect to CF servers %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	go func() {
		<-sigc
		cancel()
	}()

	ticker := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			showFeature, err := client.BoolVariation(featureFlag, &target, false)
			if err != nil {
				fmt.Printf("Error getting value: %v", err)
			}
			fmt.Printf("KeyFeature flag '%s' is %t for this user\n", featureFlag, showFeature)
		}
	}
}
