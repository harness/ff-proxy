package domain

// SSEMessage is basic object for marshalling data from ff stream
type SSEMessage struct {
	Event      string `json:"event"`
	Domain     string `json:"domain"`
	Identifier string `json:"identifier"`
	Version    int    `json:"version"`
}
