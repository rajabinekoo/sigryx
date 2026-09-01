package service

import (
	"crypto/sha256"
	"fmt"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	"github.com/rajabinekoo/sigryx/internal/cryptox"
)

type GeneratedUnseal struct {
	// Real unseal key.
	//
	// This value is ephemeral and must never be persisted or returned
	// to the owner. It will later participate in deriving the Vault
	// root encryption key.
	UnsealKey domain.UnsealKey

	// Persisted on the Sigryx side.
	ServerKeyMaterial domain.ServerKeyMaterial

	// Returned once to the owner.
	Credential domain.UnsealCredential
}

func GenerateUnseal(slotID domain.UnsealSlotID) (*GeneratedUnseal, error) {
	if slotID < 1 {
		return nil, fmt.Errorf("invalid unseal slot id: %d", slotID)
	}

	// 1. Generate the real unseal key.
	unsealKey, err := cryptox.RandomKey()
	if err != nil {
		return nil, fmt.Errorf("generate unseal key: %w", err)
	}

	// 2. Generate the owner-side secret.
	ownerSecret, err := cryptox.RandomKey()
	if err != nil {
		zeroize(unsealKey)
		return nil, fmt.Errorf("generate owner secret: %w", err)
	}

	// 3. Generate the server-side key material.
	serverKeyMaterial, err := cryptox.RandomKey()
	if err != nil {
		zeroize(unsealKey)
		zeroize(ownerSecret)

		return nil, fmt.Errorf("generate server key material: %w", err)
	}

	// 4. Derive the AES-256 wrapping key from:
	//
	// SHA256(ownerSecret || serverKeyMaterial)
	wrappingKey := deriveWrappingKey(
		ownerSecret,
		serverKeyMaterial,
	)

	defer zeroize(wrappingKey[:])

	// 5. Encrypt the real unseal key using AES-256-GCM.
	wrappedKey, err := cryptox.Seal(
		wrappingKey[:],
		unsealKey,
		unsealAAD(slotID),
	)
	if err != nil {
		zeroize(unsealKey)
		zeroize(ownerSecret)
		zeroize(serverKeyMaterial)

		return nil, fmt.Errorf("wrap unseal key: %w", err)
	}

	return &GeneratedUnseal{
		UnsealKey: domain.UnsealKey(unsealKey),

		ServerKeyMaterial: domain.ServerKeyMaterial(
			serverKeyMaterial,
		),

		Credential: domain.UnsealCredential{
			Payload: domain.UnsealPayload{
				SlotID: slotID,
				WrappedKey: domain.WrappedUnsealKey(
					wrappedKey,
				),
			},

			OwnerSecret: domain.OwnerSecret(
				ownerSecret,
			),
		},
	}, nil
}

func deriveWrappingKey(
	ownerSecret []byte,
	serverKeyMaterial []byte,
) [sha256.Size]byte {
	material := make(
		[]byte,
		0,
		len(ownerSecret)+len(serverKeyMaterial),
	)

	material = append(material, ownerSecret...)
	material = append(material, serverKeyMaterial...)

	key := sha256.Sum256(material)

	zeroize(material)

	return key
}

func unsealAAD(slotID domain.UnsealSlotID) []byte {
	return fmt.Appendf(
		nil,
		"sigryx:unseal-key:v1:%d",
		slotID,
	)
}

func zeroize(data []byte) {
	for i := range data {
		data[i] = 0
	}
}
