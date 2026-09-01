package securemem

import "errors"

var (
	ErrEmptySecret          = errors.New("securemem: secret cannot be empty")
	ErrInvalidSize          = errors.New("securemem: invalid secret size")
	ErrDestroyed            = errors.New("securemem: secret is destroyed")
	ErrSodiumInitialization = errors.New("securemem: libsodium initialization failed")
	ErrAllocation           = errors.New("securemem: secure allocation failed")
	ErrMemoryLock           = errors.New("securemem: memory locking failed")
	ErrMemoryProtection     = errors.New("securemem: memory protection failed")
)
