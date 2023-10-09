package domain

// SSEMessage is basic object for marshalling data from ff stream
type SSEMessage struct {
	Event        string   `json:"event"`
	Domain       string   `json:"domain"`
	Identifier   string   `json:"identifier"`
	Version      int      `json:"version"`
	Environment  string   `json:"environment"`
	Environments []string `json:"environments,omitempty"`
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

	EventProxyKeyDeleted    = "proxyKeyDeleted"
	EventEnvironmentAdded   = "environmentsAdded"
	EventEnvironmentRemoved = "environmentsRemoved"
	EventAPIKeyAdded        = "apiKeyAdded"
	EventAPIKeyRemoved      = "apiKeyRemoved"
)
