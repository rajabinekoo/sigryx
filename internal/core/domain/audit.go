package domain

import "time"

type AuditOutcome string

const (
	AuditOutcomeSuccess AuditOutcome = "SUCCESS"
	AuditOutcomeFailed  AuditOutcome = "FAILED"
	AuditOutcomeDenied  AuditOutcome = "DENIED"
	AuditOutcomeBlocked AuditOutcome = "BLOCKED"
)

type AuditEvent struct {
	ID         string
	OccurredAt time.Time
	ActorType  string
	ActorID    string
	SessionID  string
	Action     string
	Outcome    AuditOutcome
	SourceIP   string
	RequestID  string
	Method     string
	Path       string
	StatusCode int
	Details    map[string]any
}
