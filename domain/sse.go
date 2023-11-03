package domain

import jsoniter "github.com/json-iterator/go"

// SSEMessage is basic object for marshalling data from ff stream
type SSEMessage struct {
	Event        string   `json:"event"`
	Domain       string   `json:"domain"`
	Identifier   string   `json:"identifier"`
	Version      int      `json:"version"`
	Environment  string   `json:"environment"`
	Environments []string `json:"environments,omitempty"`
	APIKey       string   `json:"apiKey"`
}

// MarshalBinary makes SSEMessage implement the BinaryMarshaler interface
func (s SSEMessage) MarshalBinary() ([]byte, error) {
	return jsoniter.Marshal(s)
}

const (
	// MsgDomainFeature identifies flag messages from ff server or stream
	MsgDomainFeature = "flag"

	// MsgDomainSegment identifies segment messages from ff server or stream
	MsgDomainSegment = "target-segment"

	// MsgDomainProxy identifiers proxy messages from the ff server
	MsgDomainProxy = "proxy"

	// EventPatch identifies a patch event from the SSE stream
	EventPatch = "patch"

	// EventDelete identifies a delete event from the SSE stream
	EventDelete = "delete"

	// EventCreate identifies a create event from the SSE stream
	EventCreate = "create"

	// Events for proxy        proxyKeyDeleted
	EventProxyKeyDeleted    = "proxyKeyDeleted"
	EventEnvironmentAdded   = "environmentsAdded"
	EventEnvironmentRemoved = "environmentsRemoved"
	EventAPIKeyAdded        = "apiKeyAdded"
	EventAPIKeyRemoved      = "apiKeyRemoved"
)
