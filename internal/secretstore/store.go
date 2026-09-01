package secretstore

import (
	"crypto/sha256"
	"fmt"
	"sync"

	"github.com/rajabinekoo/sigryx/internal/securemem"
)

const UnsealKeySize = sha256.Size

type UnsealProgress struct {
	Submitted int
	Required  int
	Unsealed  bool
}

type Store struct {
	mu sync.RWMutex

	requiredUnsealKeys  int
	submittedUnsealKeys int

	// Index 0 is intentionally unused.
	//
	// Slot 1 -> unsealKeys[1]
	// Slot 2 -> unsealKeys[2]
	// ...
	//
	// This makes ordering explicit and deterministic.
	unsealKeys []*securemem.Secret

	// Exists only while the Vault is unsealed.
	//
	// SHA256(
	//     unsealKey[1] ||
	//     unsealKey[2] ||
	//     ... ||
	//     unsealKey[N]
	// )
	vaultEncryptionKey *securemem.Secret

	// Optional decrypted HD root seeds.
	//
	// Every seed remains inside secure memory.
	keyRootSeeds map[string]*securemem.Secret
}

func New(requiredUnsealKeys int) (*Store, error) {
	if requiredUnsealKeys < 1 {
		return nil, ErrInvalidUnsealKeyCount
	}

	return &Store{
		requiredUnsealKeys: requiredUnsealKeys,

		// +1 because index 0 is unused.
		unsealKeys: make(
			[]*securemem.Secret,
			requiredUnsealKeys+1,
		),

		keyRootSeeds: make(
			map[string]*securemem.Secret,
		),
	}, nil
}

// SubmitUnsealKey transfers ownership of key to Store.
//
// The caller MUST NOT use key after calling this method,
// regardless of whether the method succeeds or fails.
func (s *Store) SubmitUnsealKey(
	slot int,
	key *securemem.Secret,
) (UnsealProgress, error) {
	if key == nil {
		return s.Progress(), ErrNilSecret
	}

	if key.Size() != UnsealKeySize {
		key.Destroy()

		return s.Progress(), fmt.Errorf(
			"%w: expected %d bytes",
			ErrInvalidUnsealKeySize,
			UnsealKeySize,
		)
	}

	s.mu.Lock()

	if slot < 1 || slot > s.requiredUnsealKeys {
		progress := s.progressLocked()

		s.mu.Unlock()

		key.Destroy()

		return progress, fmt.Errorf(
			"%w: %d",
			ErrInvalidUnsealSlot,
			slot,
		)
	}

	if s.vaultEncryptionKey != nil {
		progress := s.progressLocked()

		s.mu.Unlock()

		key.Destroy()

		return progress, ErrVaultAlreadyUnsealed
	}

	if s.unsealKeys[slot] != nil {
		progress := s.progressLocked()

		s.mu.Unlock()

		key.Destroy()

		return progress, fmt.Errorf(
			"%w: %d",
			ErrDuplicateUnsealSlot,
			slot,
		)
	}

	s.unsealKeys[slot] = key
	s.submittedUnsealKeys++

	if s.submittedUnsealKeys < s.requiredUnsealKeys {
		progress := s.progressLocked()

		s.mu.Unlock()

		return progress, nil
	}

	//
	// Every slot is now available.
	//
	vaultKey, err := s.deriveVaultEncryptionKeyLocked()
	if err != nil {
		keys := s.takeUnsealKeysLocked()
		progress := s.progressLocked()

		s.mu.Unlock()

		destroySecrets(keys)

		return progress, fmt.Errorf(
			"derive vault encryption key: %w",
			err,
		)
	}

	s.vaultEncryptionKey = vaultKey

	//
	// Real unseal keys have completed their job.
	// Detach them from Store before destroying them.
	//
	keys := s.takeUnsealKeysLocked()

	progress := s.progressLocked()

	s.mu.Unlock()

	destroySecrets(keys)

	return progress, nil
}

// ResetUnsealAttempt destroys all currently submitted
// real unseal keys.
//
// It can only be used while the Vault is still sealed.
func (s *Store) ResetUnsealAttempt() error {
	s.mu.Lock()

	if s.vaultEncryptionKey != nil {
		s.mu.Unlock()

		return ErrVaultAlreadyUnsealed
	}

	keys := s.takeUnsealKeysLocked()

	s.mu.Unlock()

	destroySecrets(keys)

	return nil
}

func (s *Store) Progress() UnsealProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.progressLocked()
}

func (s *Store) IsUnsealed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.vaultEncryptionKey != nil
}

// WithVaultEncryptionKey provides temporary callback-only
// access to the Vault encryption key.
//
// Store's mutex is intentionally NOT held while fn executes.
// Secret itself handles synchronization and memory protection.
func (s *Store) WithVaultEncryptionKey(
	fn func([]byte) error,
) error {
	if fn == nil {
		return ErrNilCallback
	}

	s.mu.RLock()
	key := s.vaultEncryptionKey
	s.mu.RUnlock()

	if key == nil {
		return ErrVaultSealed
	}

	return key.WithBytes(fn)
}

// StoreKeyRootSeed transfers ownership of seed to Store.
//
// The seed can only be stored while the Vault is unsealed.
func (s *Store) StoreKeyRootSeed(
	rootID string,
	seed *securemem.Secret,
) error {
	if seed == nil {
		return ErrNilSecret
	}

	if rootID == "" {
		seed.Destroy()
		return ErrInvalidKeyRootID
	}

	s.mu.Lock()

	if s.vaultEncryptionKey == nil {
		s.mu.Unlock()

		seed.Destroy()

		return ErrVaultSealed
	}

	if _, exists := s.keyRootSeeds[rootID]; exists {
		s.mu.Unlock()

		seed.Destroy()

		return fmt.Errorf(
			"%w: %s",
			ErrKeyRootSeedExists,
			rootID,
		)
	}

	//
	// Ownership transfers to Store.
	//
	s.keyRootSeeds[rootID] = seed

	s.mu.Unlock()

	return nil
}

// WithKeyRootSeed provides temporary callback-only access
// to a decrypted HD root seed.
func (s *Store) WithKeyRootSeed(
	rootID string,
	fn func([]byte) error,
) error {
	if rootID == "" {
		return ErrInvalidKeyRootID
	}

	if fn == nil {
		return ErrNilCallback
	}

	s.mu.RLock()

	if s.vaultEncryptionKey == nil {
		s.mu.RUnlock()

		return ErrVaultSealed
	}

	seed := s.keyRootSeeds[rootID]

	s.mu.RUnlock()

	if seed == nil {
		return fmt.Errorf(
			"%w: %s",
			ErrKeyRootSeedNotFound,
			rootID,
		)
	}

	return seed.WithBytes(fn)
}

// Clear destroys every secret currently owned by Store.
//
// Use this during Vault sealing and process shutdown.
//
// After Clear returns:
//
//   - no pending unseal key survives
//   - no KeyRoot seed survives
//   - VaultEncryptionKey no longer exists
//   - Vault is considered sealed
//
// Store can be reused for another unseal attempt.
func (s *Store) Clear() {
	s.mu.Lock()

	unsealKeys := s.takeUnsealKeysLocked()

	vaultKey := s.vaultEncryptionKey
	s.vaultEncryptionKey = nil

	rootSeeds := s.keyRootSeeds
	s.keyRootSeeds = make(
		map[string]*securemem.Secret,
	)

	s.mu.Unlock()

	//
	// Destroy outside Store mutex.
	//
	// securemem.Secret.Destroy may wait for an active
	// WithBytes callback. Holding Store mutex here could
	// otherwise cause callback re-entry deadlocks.
	//
	destroySecrets(unsealKeys)

	for _, seed := range rootSeeds {
		if seed != nil {
			seed.Destroy()
		}
	}

	if vaultKey != nil {
		vaultKey.Destroy()
	}
}

func (s *Store) progressLocked() UnsealProgress {
	if s.vaultEncryptionKey != nil {
		return UnsealProgress{
			Submitted: s.requiredUnsealKeys,
			Required:  s.requiredUnsealKeys,
			Unsealed:  true,
		}
	}

	return UnsealProgress{
		Submitted: s.submittedUnsealKeys,
		Required:  s.requiredUnsealKeys,
		Unsealed:  false,
	}
}

// deriveVaultEncryptionKeyLocked computes:
//
// SHA256(
//
//	unsealKey[1] ||
//	unsealKey[2] ||
//	... ||
//	unsealKey[N]
//
// )
//
// Keys are consumed strictly by slot order.
//
// No concatenated plaintext buffer containing all keys
// is ever allocated.
func (s *Store) deriveVaultEncryptionKeyLocked() (
	*securemem.Secret,
	error,
) {
	hasher := sha256.New()
	defer hasher.Reset()

	for slot := 1; slot <= s.requiredUnsealKeys; slot++ {
		key := s.unsealKeys[slot]

		if key == nil {
			return nil, fmt.Errorf(
				"missing unseal key slot: %d",
				slot,
			)
		}

		err := key.WithBytes(
			func(data []byte) error {
				_, err := hasher.Write(data)
				return err
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"read unseal key slot %d: %w",
				slot,
				err,
			)
		}
	}

	//
	// Sum allocates the final 32-byte digest on the Go heap.
	//
	// securemem.New immediately copies it into protected
	// memory and wipes the supplied slice.
	//
	derived := hasher.Sum(nil)

	vaultKey, err := securemem.New(derived)
	if err != nil {
		return nil, err
	}

	return vaultKey, nil
}

// takeUnsealKeysLocked detaches all pending unseal keys
// from Store and resets the current attempt.
func (s *Store) takeUnsealKeysLocked() []*securemem.Secret {
	keys := s.unsealKeys

	s.unsealKeys = make(
		[]*securemem.Secret,
		s.requiredUnsealKeys+1,
	)

	s.submittedUnsealKeys = 0

	return keys
}

func destroySecrets(
	secrets []*securemem.Secret,
) {
	for _, secret := range secrets {
		if secret != nil {
			secret.Destroy()
		}
	}
}
