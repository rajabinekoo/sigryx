package domain

import (
	"strings"
	"time"
)

type AuditOutcome string

const (
	AuditOutcomeSuccess AuditOutcome = "SUCCESS"
	AuditOutcomeFailed  AuditOutcome = "FAILED"
	AuditOutcomeDenied  AuditOutcome = "DENIED"
	AuditOutcomeBlocked AuditOutcome = "BLOCKED"
)

type AuditRetentionClass string

const (
	AuditRetentionNormal   AuditRetentionClass = "NORMAL"
	AuditRetentionCritical AuditRetentionClass = "CRITICAL"
)

type AuditEvent struct {
	ID             string
	OccurredAt     time.Time
	ActorType      string
	ActorID        string
	SessionID      string
	Action         string
	Outcome        AuditOutcome
	SourceIP       string
	RequestID      string
	Method         string
	Path           string
	StatusCode     int
	Details        map[string]any
	RetentionClass AuditRetentionClass
}

func (e AuditEvent) EffectiveRetentionClass() AuditRetentionClass {
	switch e.RetentionClass {
	case AuditRetentionNormal, AuditRetentionCritical:
		return e.RetentionClass
	}
	return DefaultAuditRetentionClass(e.Action)
}

func DefaultAuditRetentionClass(action string) AuditRetentionClass {
	switch {
	case action == "audit.retention_cleanup",
		strings.HasPrefix(action, "security."),
		strings.HasPrefix(action, "sign."),
		strings.HasPrefix(action, "recovery."),
		strings.HasPrefix(action, "vault.") && action != "vault.status",
		strings.HasPrefix(action, "keyroot.") && action != "keyroot.list",
		strings.HasPrefix(action, "access.") && !strings.HasSuffix(action, ".list"),
		action == "auth.setup",
		action == "auth.service_token",
		action == "auth.update_me",
		action == "wallet.create":
		return AuditRetentionCritical
	default:
		return AuditRetentionNormal
	}
}
