package secretstore

import "errors"

var (
	ErrInvalidUnsealKeyCount = errors.New(
		"secretstore: invalid unseal key count",
	)

	ErrInvalidUnsealSlot = errors.New(
		"secretstore: invalid unseal slot",
	)

	ErrInvalidUnsealKeySize = errors.New(
		"secretstore: invalid unseal key size",
	)

	ErrDuplicateUnsealSlot = errors.New(
		"secretstore: unseal slot already submitted",
	)

	ErrNilSecret = errors.New(
		"secretstore: secret is nil",
	)

	ErrNilCallback = errors.New(
		"secretstore: callback is nil",
	)

	ErrVaultAlreadyUnsealed = errors.New(
		"secretstore: vault is already unsealed",
	)

	ErrVaultSealed = errors.New(
		"secretstore: vault is sealed",
	)

	ErrInvalidKeyRootID = errors.New(
		"secretstore: invalid key root id",
	)

	ErrKeyRootSeedExists = errors.New(
		"secretstore: key root seed already exists",
	)

	ErrKeyRootSeedNotFound = errors.New(
		"secretstore: key root seed not found",
	)
)
