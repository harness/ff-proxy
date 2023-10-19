package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/harness/ff-proxy/v2/config"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

var (
	// ErrUnexpectedMessageDomain is the error returned when an SSE message has a message domain we aren't expecting
	ErrUnexpectedMessageDomain = errors.New("unexpected message domain")

	// ErrUnexpectedEventType is the error returned when an SSE message has an event type we aren't expecting
	ErrUnexpectedEventType = errors.New("unexpected event type")
)

// Refresher is a type for handling SSE events from Harness Saas
type Refresher struct {
	proxyKey          string
	authToken         string
	clusterIdentifier string
	log               log.Logger
	clientService     domain.ClientService
	config            config.Config
	proxyConfig       []domain.ProxyConfig
	authRepo          domain.AuthRepo
	flagRepo          domain.FlagRepo
	segmentRepo       domain.SegmentRepo
}

// NewRefresher creates a Refresher
func NewRefresher(l log.Logger, config config.Config, client domain.ClientService, authRepo domain.AuthRepo, flagRepo domain.FlagRepo, segmentRepo domain.SegmentRepo) Refresher {
	l = l.With("component", "Refresher")
	return Refresher{log: l, config: config, clientService: client, authRepo: authRepo, flagRepo: flagRepo, segmentRepo: segmentRepo}
}

// HandleMessage makes Refresher implement the MessageHandler interface
func (s Refresher) HandleMessage(ctx context.Context, msg domain.SSEMessage) error {
	switch msg.Domain {
	case domain.MsgDomainFeature:
		return handleFeatureMessage(ctx, msg)
	case domain.MsgDomainSegment:
		return handleSegmentMessage(ctx, msg)
	case domain.MsgDomainProxy:
		return s.handleProxyMessage(ctx, msg)
	default:
		return fmt.Errorf("%w: %s", ErrUnexpectedMessageDomain, msg.Domain)
	}

}

func handleFeatureMessage(_ context.Context, msg domain.SSEMessage) error {
	switch msg.Event {
	case domain.EventDelete:
		// delete from the cache
	case domain.EventPatch, domain.EventCreate:

	default:
		return fmt.Errorf("%w %q for FeatureMessage", ErrUnexpectedEventType, msg.Event)
	}
	return nil
}

func handleSegmentMessage(_ context.Context, msg domain.SSEMessage) error {
	switch msg.Event {
	case domain.EventDelete:
	case domain.EventPatch, domain.EventCreate:

	default:
		return fmt.Errorf("%w %q for SegmentMessage", ErrUnexpectedEventType, msg.Event)
	}
	return nil
}

func (s Refresher) handleProxyMessage(ctx context.Context, msg domain.SSEMessage) error {
	switch msg.Event {
	case domain.EventProxyKeyDeleted:
		// todo
		return nil
	case domain.EventEnvironmentAdded:
		if err := s.handleAddEnvironmentEvent(ctx, msg.Environments); err != nil {
			s.log.Error("failed to handle addEnvironmentEvent", "err", err)
			return err
		}
	case domain.EventEnvironmentRemoved:
		// todo
		return nil
	case domain.EventAPIKeyAdded:
		// todo
		return nil
	case domain.EventAPIKeyRemoved:
		// todo
		return nil
	default:
		return fmt.Errorf("%w %q for Proxymessage", ErrUnexpectedEventType, msg.Event)
	}
	return nil
}

// handleAddEnvironmentEvent fetches proxyConfig for all added environments and sets them on.
func (s Refresher) handleAddEnvironmentEvent(ctx context.Context, environments []string) error {
	for _, env := range environments {
		input := domain.GetProxyConfigInput{
			Key:               s.config.Key(),
			EnvID:             env,
			AuthToken:         s.config.Token(),
			ClusterIdentifier: s.config.ClusterIdentifier(),
			PageNumber:        0,
			PageSize:          10,
		}

		proxyConfig, err := s.clientService.PageProxyConfig(ctx, input)
		if err != nil {
			s.log.Error("unable to fetch config for the environment", "environment", env)
			return err
		}
		s.config.SetProxyConfig(proxyConfig)
		if err := s.config.Populate(ctx, s.authRepo, s.flagRepo, s.segmentRepo); err != nil {
			return err
		}
	}
	return nil
}
