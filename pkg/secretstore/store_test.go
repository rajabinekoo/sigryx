package secretstore

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/rajabinekoo/sigryx/pkg/securemem"
)

func TestNewRejectsInvalidUnsealKeyCount(t *testing.T) {
	_, err := New(0)

	if !errors.Is(err, ErrInvalidUnsealKeyCount) {
		t.Fatalf(
			"expected ErrInvalidUnsealKeyCount, got %v",
			err,
		)
	}
}

func TestSubmitUnsealKeysDerivesVaultKeyBySlotOrder(
	t *testing.T,
) {
	store, err := New(3)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Clear()

	k1 := repeatedKey(0x11)
	k2 := repeatedKey(0x22)
	k3 := repeatedKey(0x33)

	s1 := mustSecret(t, k1)
	s2 := mustSecret(t, k2)
	s3 := mustSecret(t, k3)

	//
	// Deliberately submit out of order.
	//
	progress, err := store.SubmitUnsealKey(3, s3)
	if err != nil {
		t.Fatal(err)
	}

	assertProgress(t, progress, 1, 3, false)

	progress, err = store.SubmitUnsealKey(1, s1)
	if err != nil {
		t.Fatal(err)
	}

	assertProgress(t, progress, 2, 3, false)

	progress, err = store.SubmitUnsealKey(2, s2)
	if err != nil {
		t.Fatal(err)
	}

	assertProgress(t, progress, 3, 3, true)

	if !store.IsUnsealed() {
		t.Fatal("store should be unsealed")
	}

	//
	// Real unseal keys must be gone immediately
	// after VaultEncryptionKey derivation.
	//
	if !s1.IsDestroyed() {
		t.Fatal("slot 1 key was not destroyed")
	}

	if !s2.IsDestroyed() {
		t.Fatal("slot 2 key was not destroyed")
	}

	if !s3.IsDestroyed() {
		t.Fatal("slot 3 key was not destroyed")
	}

	expected := deriveExpectedVaultKey(
		k1,
		k2,
		k3,
	)

	err = store.WithVaultEncryptionKey(
		func(actual []byte) error {
			if !bytes.Equal(actual, expected) {
				t.Fatal(
					"vault encryption key was not derived in slot order",
				)
			}

			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	//
	// Progress must remain completed even though
	// individual unseal keys were destroyed.
	//
	progress = store.Progress()

	assertProgress(t, progress, 3, 3, true)
}

func TestDuplicateSlotIsRejectedAndIncomingKeyDestroyed(
	t *testing.T,
) {
	store, err := New(2)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Clear()

	first := mustSecret(
		t,
		repeatedKey(0x11),
	)

	duplicate := mustSecret(
		t,
		repeatedKey(0x22),
	)

	_, err = store.SubmitUnsealKey(1, first)
	if err != nil {
		t.Fatal(err)
	}

	progress, err := store.SubmitUnsealKey(
		1,
		duplicate,
	)

	if !errors.Is(err, ErrDuplicateUnsealSlot) {
		t.Fatalf(
			"expected ErrDuplicateUnsealSlot, got %v",
			err,
		)
	}

	if !duplicate.IsDestroyed() {
		t.Fatal("duplicate key should be destroyed")
	}

	if first.IsDestroyed() {
		t.Fatal("accepted key should still be pending")
	}

	assertProgress(t, progress, 1, 2, false)
}

func TestInvalidSlotDestroysIncomingKey(t *testing.T) {
	store, err := New(3)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Clear()

	key := mustSecret(
		t,
		repeatedKey(0x11),
	)

	_, err = store.SubmitUnsealKey(4, key)

	if !errors.Is(err, ErrInvalidUnsealSlot) {
		t.Fatalf(
			"expected ErrInvalidUnsealSlot, got %v",
			err,
		)
	}

	if !key.IsDestroyed() {
		t.Fatal("rejected key should be destroyed")
	}
}

func TestInvalidUnsealKeySizeIsRejected(t *testing.T) {
	store, err := New(1)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Clear()

	key := mustSecret(
		t,
		[]byte("too-short"),
	)

	_, err = store.SubmitUnsealKey(1, key)

	if !errors.Is(err, ErrInvalidUnsealKeySize) {
		t.Fatalf(
			"expected ErrInvalidUnsealKeySize, got %v",
			err,
		)
	}

	if !key.IsDestroyed() {
		t.Fatal("invalid key should be destroyed")
	}
}

func TestResetUnsealAttemptDestroysPendingKeys(
	t *testing.T,
) {
	store, err := New(3)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Clear()

	k1 := mustSecret(t, repeatedKey(0x11))
	k2 := mustSecret(t, repeatedKey(0x22))

	if _, err := store.SubmitUnsealKey(1, k1); err != nil {
		t.Fatal(err)
	}

	if _, err := store.SubmitUnsealKey(2, k2); err != nil {
		t.Fatal(err)
	}

	if err := store.ResetUnsealAttempt(); err != nil {
		t.Fatal(err)
	}

	if !k1.IsDestroyed() {
		t.Fatal("slot 1 key should be destroyed")
	}

	if !k2.IsDestroyed() {
		t.Fatal("slot 2 key should be destroyed")
	}

	assertProgress(
		t,
		store.Progress(),
		0,
		3,
		false,
	)
}

func TestResetUnsealAttemptFailsAfterUnseal(
	t *testing.T,
) {
	store := mustUnsealedStore(t)
	defer store.Clear()

	err := store.ResetUnsealAttempt()

	if !errors.Is(err, ErrVaultAlreadyUnsealed) {
		t.Fatalf(
			"expected ErrVaultAlreadyUnsealed, got %v",
			err,
		)
	}
}

func TestWithVaultEncryptionKeyFailsWhileSealed(
	t *testing.T,
) {
	store, err := New(1)
	if err != nil {
		t.Fatal(err)
	}

	err = store.WithVaultEncryptionKey(
		func([]byte) error {
			return nil
		},
	)

	if !errors.Is(err, ErrVaultSealed) {
		t.Fatalf(
			"expected ErrVaultSealed, got %v",
			err,
		)
	}
}

func TestVaultEncryptionKeyCallbackErrorIsPropagated(
	t *testing.T,
) {
	store := mustUnsealedStore(t)
	defer store.Clear()

	expected := errors.New("callback failed")

	err := store.WithVaultEncryptionKey(
		func([]byte) error {
			return expected
		},
	)

	if !errors.Is(err, expected) {
		t.Fatalf(
			"expected callback error, got %v",
			err,
		)
	}
}

func TestStoreKeyRootSeedFailsWhileSealed(
	t *testing.T,
) {
	store, err := New(1)
	if err != nil {
		t.Fatal(err)
	}

	seed := mustSecret(
		t,
		[]byte("master-seed"),
	)

	err = store.StoreKeyRootSeed(
		"root-1",
		seed,
	)

	if !errors.Is(err, ErrVaultSealed) {
		t.Fatalf(
			"expected ErrVaultSealed, got %v",
			err,
		)
	}

	if !seed.IsDestroyed() {
		t.Fatal("rejected seed should be destroyed")
	}
}

func TestKeyRootSeedCanBeStoredAndRead(
	t *testing.T,
) {
	store := mustUnsealedStore(t)
	defer store.Clear()

	expected := []byte(
		"this-is-a-master-seed",
	)

	seed := mustSecret(
		t,
		expected,
	)

	if err := store.StoreKeyRootSeed(
		"root-1",
		seed,
	); err != nil {
		t.Fatal(err)
	}

	err := store.WithKeyRootSeed(
		"root-1",
		func(actual []byte) error {
			if !bytes.Equal(actual, expected) {
				t.Fatal("unexpected key root seed")
			}

			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDuplicateKeyRootSeedIsRejected(
	t *testing.T,
) {
	store := mustUnsealedStore(t)
	defer store.Clear()

	first := mustSecret(
		t,
		[]byte("seed-one"),
	)

	second := mustSecret(
		t,
		[]byte("seed-two"),
	)

	if err := store.StoreKeyRootSeed(
		"root-1",
		first,
	); err != nil {
		t.Fatal(err)
	}

	err := store.StoreKeyRootSeed(
		"root-1",
		second,
	)

	if !errors.Is(err, ErrKeyRootSeedExists) {
		t.Fatalf(
			"expected ErrKeyRootSeedExists, got %v",
			err,
		)
	}

	if !second.IsDestroyed() {
		t.Fatal(
			"duplicate seed should be destroyed",
		)
	}

	if first.IsDestroyed() {
		t.Fatal(
			"existing seed should remain valid",
		)
	}
}

func TestClearDestroysAllRuntimeSecrets(
	t *testing.T,
) {
	store := mustUnsealedStore(t)

	seed := mustSecret(
		t,
		[]byte("master-seed"),
	)

	if err := store.StoreKeyRootSeed(
		"root-1",
		seed,
	); err != nil {
		t.Fatal(err)
	}

	store.Clear()

	if store.IsUnsealed() {
		t.Fatal("store should be sealed after Clear")
	}

	if !seed.IsDestroyed() {
		t.Fatal(
			"key root seed should be destroyed",
		)
	}

	assertProgress(
		t,
		store.Progress(),
		0,
		1,
		false,
	)

	err := store.WithVaultEncryptionKey(
		func([]byte) error {
			return nil
		},
	)

	if !errors.Is(err, ErrVaultSealed) {
		t.Fatalf(
			"expected ErrVaultSealed, got %v",
			err,
		)
	}
}

// This test catches an important locking regression.
//
// WithVaultEncryptionKey must not keep Store.mu locked while
// executing the callback, otherwise calling another Store
// operation from inside the callback would deadlock.
func TestVaultKeyCallbackCanReenterStore(
	t *testing.T,
) {
	store := mustUnsealedStore(t)
	defer store.Clear()

	seed := mustSecret(
		t,
		[]byte("master-seed"),
	)

	done := make(chan error, 1)

	go func() {
		done <- store.WithVaultEncryptionKey(
			func([]byte) error {
				return store.StoreKeyRootSeed(
					"root-1",
					seed,
				)
			},
		)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}

	case <-time.After(time.Second):
		t.Fatal(
			"possible deadlock while re-entering Store from callback",
		)
	}
}

func mustUnsealedStore(t *testing.T) *Store {
	t.Helper()

	store, err := New(1)
	if err != nil {
		t.Fatal(err)
	}

	key := mustSecret(
		t,
		repeatedKey(0x42),
	)

	progress, err := store.SubmitUnsealKey(
		1,
		key,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !progress.Unsealed {
		t.Fatal("store did not become unsealed")
	}

	return store
}

func mustSecret(
	t *testing.T,
	data []byte,
) *securemem.Secret {
	t.Helper()

	//
	// securemem.New takes ownership and wipes its input,
	// therefore pass it an independent copy.
	//
	input := append(
		[]byte(nil),
		data...,
	)

	secret, err := securemem.New(input)
	if err != nil {
		t.Fatal(err)
	}

	return secret
}

func repeatedKey(value byte) []byte {
	return bytes.Repeat(
		[]byte{value},
		UnsealKeySize,
	)
}

func deriveExpectedVaultKey(
	keys ...[]byte,
) []byte {
	hasher := sha256.New()

	for _, key := range keys {
		_, _ = hasher.Write(key)
	}

	return hasher.Sum(nil)
}

func assertProgress(
	t *testing.T,
	progress UnsealProgress,
	submitted int,
	required int,
	unsealed bool,
) {
	t.Helper()

	if progress.Submitted != submitted {
		t.Fatalf(
			"expected %d submitted, got %d",
			submitted,
			progress.Submitted,
		)
	}

	if progress.Required != required {
		t.Fatalf(
			"expected %d required, got %d",
			required,
			progress.Required,
		)
	}

	if progress.Unsealed != unsealed {
		t.Fatalf(
			"expected unsealed=%v, got %v",
			unsealed,
			progress.Unsealed,
		)
	}
}
