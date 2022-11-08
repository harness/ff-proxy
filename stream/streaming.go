package stream

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// Stream defines the interface for writing and reading domain.StreamEvents
// from a Stream
type Stream interface {
	// Pub publishes an event to a given topic on the stream
	Pub(ctx context.Context, topic string, event StreamEvent) error
	// Sub subscribes to a given topic on the stream and waits for events
	Sub(ctx context.Context, topic string, checkpoint string, onReceive func(StreamEvent)) error
}

var checkpointRegex = regexp.MustCompile(`(\d{13}-\d{1})`)

// Checkpoint represents a checkpoint in a Redis stream. The format of a
// checkpoints is <timestamp>-<sequence num> e.g. 1643296917414-0
type Checkpoint string

// NewCheckpoint returns a NewCheckpoint
func NewCheckpoint(s string) (Checkpoint, error) {
	if ok := checkpointRegex.MatchString(s); !ok {
		return Checkpoint(""), fmt.Errorf("string does not match expected format for checkpoint, got: %s, expected <timestamp>-<sequence num> e.g 1643296917414-0", s)
	}
	return Checkpoint(s), nil
}

// IsOlder checks whether the passed checkpoint is older than the current one
func (c Checkpoint) IsOlder(other Checkpoint) bool {
	newCheckpoint := strings.Split(string(c), "-")
	oldCheckpoint := strings.Split(string(other), "-")

	if newCheckpoint[0] == oldCheckpoint[0] {
		return newCheckpoint[1] < oldCheckpoint[1]
	}

	return newCheckpoint[0] < oldCheckpoint[0]
}

// StreamEventValue is the key type for the Values map in a StreamEvent
type StreamEventValue string

// String makes StreamEventValue implement the stringer interface
func (s StreamEventValue) String() string {
	return string(s)
}

const (
	// StreamEventValueAPIKey is the Key for the HashedAPIKey
	StreamEventValueAPIKey StreamEventValue = "HashedAPIKey"
	// StreamEventValueData is the key for the Data
	StreamEventValueData StreamEventValue = "Data"
)

// StreamEvent is the type that's written to the stream by the EventListener
// and received by the StreamWorker. The heckpoint will be populated for events
// received from the stream. If you're publishing a StreamEvent there's no need
// to populate the Checkpoint field.
type StreamEvent struct {
	Checkpoint Checkpoint
	Values     map[StreamEventValue]string
}

// NewStreamEvent creates a StreamEvent
func NewStreamEvent(values map[StreamEventValue]string) StreamEvent {
	return StreamEvent{Values: values}
}

// NewStreamEventFromMap creates a StreamEvent from a map[string]interface{}
func NewStreamEventFromMap(values map[string]interface{}) (StreamEvent, error) {
	apiKey, ok := values[string(StreamEventValueAPIKey)]
	if !ok {
		return StreamEvent{}, fmt.Errorf("key %q not found in values map", StreamEventValueAPIKey)
	}

	data, ok := values[string(StreamEventValueData)]
	if !ok {
		return StreamEvent{}, fmt.Errorf("key %q not found in values map", StreamEventValueData)
	}

	sapiKey, ok := apiKey.(string)
	if !ok {
		return StreamEvent{}, fmt.Errorf("unexpected type for %s value, expected %T got %T", StreamEventValueAPIKey, "", sapiKey)
	}

	sdata, ok := data.(string)
	if !ok {
		return StreamEvent{}, fmt.Errorf("unexpected type for %s, expected %T got %T", StreamEventValueData, "", sapiKey)
	}

	return StreamEvent{
		Values: map[StreamEventValue]string{
			StreamEventValueAPIKey: sapiKey,
			StreamEventValueData:   sdata,
		},
	}, nil
}
