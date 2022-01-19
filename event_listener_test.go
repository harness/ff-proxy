package ffproxy

import (
	"context"
	"testing"

	"github.com/harness/ff-golang-server-sdk/stream"
	"github.com/harness/ff-proxy/log"
	"github.com/stretchr/testify/assert"
	"github.com/wings-software/ff-server/pkg/hash"
)

func TestEventListener_Pub(t *testing.T) {
	el := NewEventListener(log.NoOpLogger{}, nil, hash.NewSha256())
	ctx := context.Background()
	event := stream.Event{}

	err := el.Pub(ctx, event)
	assert.Nil(t, err)
}
