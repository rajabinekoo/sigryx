// Package errors defines transport-agnostic error kinds shared across services.
// Domain errors map to a Kind; adapters translate Kind → HTTP status / gRPC code
// at the edge. Keep this list small and stable.
package errors

import (
	stderrors "errors"
	"fmt"
)

// Kind classifies an error for transport-layer translation.
type Kind int

const (
	KindUnknown Kind = iota
	KindInvalidInput
	KindUnauthenticated
	KindPermissionDenied
	KindNotFound
	KindAlreadyExists
	KindConflict
	KindRateLimited
	KindInternal
	KindUnavailable
)

func (k Kind) String() string {
	switch k {
	case KindInvalidInput:
		return "invalid_input"
	case KindUnauthenticated:
		return "unauthenticated"
	case KindPermissionDenied:
		return "permission_denied"
	case KindNotFound:
		return "not_found"
	case KindAlreadyExists:
		return "already_exists"
	case KindConflict:
		return "conflict"
	case KindRateLimited:
		return "rate_limited"
	case KindInternal:
		return "internal"
	case KindUnavailable:
		return "unavailable"
	default:
		return "unknown"
	}
}

// Error carries a Kind plus the underlying cause. Use New / Wrap to construct.
type Error struct {
	Kind    Kind
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	if e.Message == "" {
		return e.Cause.Error()
	}
	return fmt.Sprintf("%s: %s", e.Message, e.Cause.Error())
}

func (e *Error) Unwrap() error { return e.Cause }

// New builds an Error without a cause.
func New(kind Kind, msg string) *Error {
	return &Error{Kind: kind, Message: msg}
}

// Wrap attaches kind+message to an existing cause.
func Wrap(kind Kind, msg string, cause error) *Error {
	return &Error{Kind: kind, Message: msg, Cause: cause}
}

// KindOf returns the Kind for any error, walking Unwrap chains. Returns
// KindUnknown for errors that don't carry one.
func KindOf(err error) Kind {
	if err == nil {
		return KindUnknown
	}
	var e *Error
	if stderrors.As(err, &e) {
		return e.Kind
	}
	return KindUnknown
}
