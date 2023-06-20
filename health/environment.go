package health

import (
	"context"
	"sync"
	"time"

	"github.com/harness/ff-proxy/domain"
)

type streamStatuser interface {
	StreamConnected(envID string) bool
}

// EnvironmentHealthTracker tracks the health of environments
type EnvironmentHealthTracker struct {
	*sync.RWMutex
	environments map[string]domain.StreamStatus
	streamStatus streamStatuser
}

// NewEnvironmentHealthTracker creates an EnvironmentHealthTracker
func NewEnvironmentHealthTracker(ctx context.Context, envs []string, s streamStatuser, t time.Duration) func() []domain.EnvironmentHealth {
	em := make(map[string]domain.StreamStatus)
	for _, envID := range envs {
		em[envID] = domain.StreamStatus{
			State: domain.StreamStateInitializing,
			Since: time.Now().Unix(),
		}
	}

	e := EnvironmentHealthTracker{
		RWMutex:      &sync.RWMutex{},
		environments: em,
		streamStatus: s,
	}

	e.trackHealthStatus(ctx, t)
	return e.GetEnvHealth
}

func (e EnvironmentHealthTracker) trackHealthStatus(ctx context.Context, pollDuration time.Duration) {
	go func() {
		ticker := time.NewTicker(pollDuration)
		defer ticker.Stop()

		checkStatus := func() {
			e.Lock()
			defer e.Unlock()

			for envID, lastStreamStatus := range e.environments {
				currentState := boolToStreamState(e.streamStatus.StreamConnected(envID))

				// If the last status and current status States are the same then there's been no change
				// so we don't need to modify anything
				if lastStreamStatus.State == currentState {
					continue
				}

				// If they don't match then something has changed so we should modify the state
				e.environments[envID] = domain.StreamStatus{
					State: currentState,
					Since: time.Now().Unix()}
			}
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkStatus()
			}
		}
	}()
}

// GetEnvHealth is a function that checks the health of all connected environments
func (e EnvironmentHealthTracker) GetEnvHealth() []domain.EnvironmentHealth {
	e.RLock()
	defer e.RUnlock()

	result := make([]domain.EnvironmentHealth, 0, len(e.environments))
	for envID, streamStatus := range e.environments {
		result = append(result, domain.EnvironmentHealth{
			ID:           envID,
			StreamStatus: streamStatus,
		})
	}

	return result
}

func boolToStreamState(b bool) domain.StreamState {
	if b {
		return domain.StreamStateConnected
	}
	return domain.StreamStateDisconnected
}
