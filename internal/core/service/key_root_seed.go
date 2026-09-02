package service

import (
	"errors"
	"fmt"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	"github.com/rajabinekoo/sigryx/pkg/cryptox"
	"github.com/rajabinekoo/sigryx/pkg/secretstore"
	"github.com/rajabinekoo/sigryx/pkg/securemem"
)

func withKeyRootSeed(
	secrets *secretstore.Store,
	root *domain.KeyRoot,
	fn func([]byte) error,
) error {
	err := secrets.WithKeyRootSeed(root.ID, fn)
	if err == nil {
		return nil
	}
	if !errors.Is(err, secretstore.ErrKeyRootSeedNotFound) {
		return err
	}

	var plaintext []byte
	err = secrets.WithVaultEncryptionKey(func(vaultKey []byte) error {
		var openErr error
		plaintext, openErr = cryptox.Open(
			vaultKey,
			cryptox.SealedPayload(root.SealedSeed),
			keyRootAAD(root.ID, root.DerivationScheme),
		)
		return openErr
	})
	if err != nil {
		return fmt.Errorf("open key root seed: %w", err)
	}

	seed, err := securemem.New(plaintext)
	if err != nil {
		return fmt.Errorf("protect key root seed: %w", err)
	}

	if err := secrets.StoreKeyRootSeed(root.ID, seed); err != nil {
		if !errors.Is(err, secretstore.ErrKeyRootSeedExists) {
			return fmt.Errorf("cache key root seed: %w", err)
		}
	}

	return secrets.WithKeyRootSeed(root.ID, fn)
}
