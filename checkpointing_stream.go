package ffproxy

import (
	"context"
	"fmt"

	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/log"
)

// Checkpointer is a type for storing checkpoints
type Checkpointer interface {
	SetKV(ctx context.Context, key string, value string) error
	GetKV(ctx context.Context, key string) (string, error)
}

type checkpoint struct {
	key   string
	value domain.Checkpoint
}

func newCheckpoint(key string, value domain.Checkpoint) checkpoint {
	return checkpoint{key: key, value: value}
}

type checkpoints chan checkpoint

// CheckpointingStream is a stream that stores checkpoints of the last event
// processed so it can resume from this point in the event of a failure.
type CheckpointingStream struct {
	stream      domain.Stream
	checkpoint  Checkpointer
	log         log.Logger
	checkpoints checkpoints
}

// NewCheckpointingStream creates a CheckpointingStream and starts a process
// that listens for checkpoints
func NewCheckpointingStream(ctx context.Context, s domain.Stream, c Checkpointer, l log.Logger) CheckpointingStream {
	l = l.With("component", "StreamWorker")
	sc := CheckpointingStream{stream: s, checkpoint: c, log: l, checkpoints: make(checkpoints)}
	sc.setCheckpoints(ctx)
	return sc
}

// setCheckpoint listens for checkpoints and stores them in the Checkpointer. It
// does a check to make sure it only stores checkpoints that are newer than the
// checkpoint that's already stored. It will run until the context has been canceled
func (s CheckpointingStream) setCheckpoints(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				s.log.Info("context canceled, exiting setCheckpoint goroutine")
				return
			case c, ok := <-s.checkpoints:
				if !ok {
					s.log.Info("checkpoints channel closed")
					return
				}

				oldCheckpoint := s.fetchCheckpoint(ctx, c.key)
				if c.value.IsOlder(domain.Checkpoint(oldCheckpoint)) {
					continue
				}

				s.log.Info("setting checkpoint", "key", c.key, "checkpoint", c.value)
				s.checkpoint.SetKV(ctx, c.key, string(c.value))
			}
		}
	}()
}

// Pub makes CheckpointingStream implement the Stream interface.
func (s CheckpointingStream) Pub(ctx context.Context, topic string, values domain.StreamEvent) error {
	return s.stream.Pub(ctx, topic, values)
}

// Sub makes CheckpointingStream implement the Stream interface. It subscribes
// to the stream at the given checkpoint. If no checkpoint is provided it will
// attempt to fetch the last known checkpoint and use it. If it fails to find a
// checkpoint then the point at which it begins subscribing to the stream is determined
// by the implementation of the underlying Stream that the CheckpointingStream
// uses.
func (s CheckpointingStream) Sub(ctx context.Context, topic string, checkpoint string, onReceive func(domain.StreamEvent)) error {
	key := fmt.Sprintf("checkpoint-%s", topic)

	if checkpoint == "" {
		checkpoint = s.fetchCheckpoint(ctx, topic)
	}

	err := s.stream.Sub(ctx, topic, checkpoint, func(e domain.StreamEvent) {
		onReceive(e)

		select {
		case <-ctx.Done():
			return
		case s.checkpoints <- newCheckpoint(key, e.Checkpoint):
		}
	})
	if err != nil {
		return err
	}

	return nil
}

// fetchCheckpoint is a helper for fetching a checkpoint
func (s CheckpointingStream) fetchCheckpoint(ctx context.Context, key string) string {
	checkpoint, err := s.checkpoint.GetKV(ctx, key)
	if err != nil {
		// Log error but continue with empty checkpoint
		s.log.Error("failed to fetch checkpoint, continuing with default checkpoint value \"\"", "checkpoint_key", key, "err", err)
		return ""
	}
	return checkpoint
}
