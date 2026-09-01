package securemem

import (
	"sync"
)

type Secret struct {
	mu sync.Mutex

	region    *region
	destroyed bool
}

// New moves sensitive bytes into protected memory.
//
// Ownership of data is transferred to Secret.
// The input slice is wiped before New returns.
func New(data []byte) (*Secret, error) {
	if len(data) == 0 {
		return nil, ErrEmptySecret
	}

	//
	// From this point on, even when allocation fails,
	// the caller-provided plaintext must be wiped.
	//
	defer wipeGoBytes(data)

	region, err := newRegionFromBytes(data)
	if err != nil {
		return nil, err
	}

	return &Secret{
		region: region,
	}, nil
}

// Random creates a random secret directly inside protected memory.
//
// Unlike:
//
//	cryptox.RandomKey()
//	securemem.New(...)
//
// this function never creates the plaintext secret on the Go heap.
func Random(size int) (*Secret, error) {
	region, err := newRandomRegion(size)
	if err != nil {
		return nil, err
	}

	return &Secret{
		region: region,
	}, nil
}

// WithBytes provides temporary read-only access to secret material.
//
// The []byte passed to fn:
//   - MUST NOT escape the callback
//   - MUST NOT be stored
//   - MUST NOT be passed to another goroutine
//   - MUST NOT be modified
//
// Memory becomes inaccessible immediately after fn returns.
func (s *Secret) WithBytes(
	fn func([]byte) error,
) error {
	if fn == nil {
		return nil
	}

	//
	// Exclusive lock is intentional.
	//
	// mprotect changes permissions for the underlying pages.
	// Multiple concurrent readers would race while changing
	// NOACCESS <-> READONLY.
	//
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.destroyed || s.region == nil {
		return ErrDestroyed
	}

	return s.region.withBytes(fn)
}

// Destroy permanently invalidates this Secret.
//
// It is safe to call Destroy multiple times.
func (s *Secret) Destroy() {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.destroyed {
		return
	}

	if s.region != nil {
		s.region.destroy()
		s.region = nil
	}

	s.destroyed = true
}

func (s *Secret) IsDestroyed() bool {
	if s == nil {
		return true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.destroyed
}

func (s *Secret) Size() int {
	if s == nil {
		return 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.destroyed || s.region == nil {
		return 0
	}

	return s.region.size
}
