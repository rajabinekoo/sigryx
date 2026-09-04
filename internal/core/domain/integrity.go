package domain

import "time"

type SigningRecord struct {
	ID              string
	Context         string
	ObjectID        string
	EncryptedRecord []byte
	CreatedAt       time.Time
}

type Alert struct {
	Severity   string         `json:"severity"`
	Code       string         `json:"code"`
	OccurredAt time.Time      `json:"occurred_at"`
	ActorID    string         `json:"actor_id,omitempty"`
	SourceIP   string         `json:"source_ip,omitempty"`
	RequestID  string         `json:"request_id,omitempty"`
	Context    string         `json:"context,omitempty"`
	ObjectID   string         `json:"object_id,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}
