package health

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/harness/ff-proxy/v2/log"
	"github.com/hashicorp/go-multierror"
)

// Heartbeat kicks off a goroutine that polls the /health endpoint at intervals
// determined by how frequently events are sent on the tick channel.
func Heartbeat(ctx context.Context, heartbeatInterval int, listenAddr string, logger log.StructuredLogger) {
	go func() {
		ticker := time.NewTicker(time.Duration(heartbeatInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info("stopping heartbeat")
				return
			case <-ticker.C:
				resp, err := http.Get(fmt.Sprintf("%s/health", listenAddr))
				if err != nil {
					logger.Error(fmt.Sprintf("heartbeat request failed: %s", err))
					continue
				}

				if resp == nil {
					continue
				}

				if resp.StatusCode == http.StatusOK {
					logger.Info(fmt.Sprintf("heartbeat healthy: status code: %d", resp.StatusCode))
					resp.Body.Close()
					continue
				}

				b, err := io.ReadAll(resp.Body)
				if err != nil {
					resp.Body.Close()
					logger.Error(fmt.Sprintf("failed to read response body from %s", resp.Request.URL.String()))
					logger.Error(fmt.Sprintf("heartbeat unhealthy: status code: %d", resp.StatusCode))
					continue
				}
				resp.Body.Close()

				logger.Error(fmt.Sprintf("heartbeat unhealthy: status code: %d, body: %s", resp.StatusCode, b))
			}
		}
	}()
}

// StreamHealthCheck checks if the GripControl stream is available - this is required to enable streaming mode
func StreamHealthCheck() error {
	var multiErr error

	for i := 0; i < 2; i++ {
		resp, err := http.Get("http://localhost:5561")
		if err != nil || resp == nil {
			multiErr = multierror.Append(fmt.Errorf("gpc request failed streaming will be disabled: %s", err), multiErr)
		}

		defer resp.Body.Close()

		if resp != nil && resp.StatusCode != http.StatusOK {
			multiErr = multierror.Append(fmt.Errorf("gpc request failed streaming will be disabled: %d", resp.StatusCode), multiErr)
		} else {
			return nil
		}
		time.Sleep(2 * time.Second)
	}

	return multiErr
}
