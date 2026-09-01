package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/pkg/cryptox"
	"github.com/rajabinekoo/sigryx/pkg/secretstore"
	"github.com/rajabinekoo/sigryx/pkg/securemem"
)

const unsealKeySize = cryptox.KeySize

var (
	ErrInvalidUnsealKeyCount = errors.New("seal: invalid unseal key count")
	ErrNotInitialized        = errors.New("seal: vault is not initialized")
	ErrInvalidCredential     = errors.New("seal: invalid unseal credential")
	ErrCorruptedUnsealSlot   = errors.New("seal: corrupted persisted unseal slot")
)

type SealService struct {
	unsealRepository portout.UnsealKeySlotRepository
	secrets          *secretstore.Store
	maxUnsealKeys    int
}

func NewSealService(
	unsealRepository portout.UnsealKeySlotRepository,
	secrets *secretstore.Store,
	maxUnsealKeys int,
) *SealService {
	return &SealService{
		unsealRepository: unsealRepository,
		secrets:          secrets,
		maxUnsealKeys:    maxUnsealKeys,
	}
}

func (s *SealService) Initialize(
	ctx context.Context,
	input portin.InitializeVaultInput,
) (*portin.InitializeVaultResult, error) {
	if input.UnsealKeyCount < 1 ||
		(s.maxUnsealKeys > 0 && input.UnsealKeyCount > s.maxUnsealKeys) {
		return nil, ErrInvalidUnsealKeyCount
	}

	count, err := s.unsealRepository.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count unseal key slots: %w", err)
	}
	if count != 0 {
		return nil, portout.ErrAlreadyInitialized
	}

	slots := make([]domain.UnsealKeySlot, 0, input.UnsealKeyCount)
	credentials := make([]domain.UnsealCredential, 0, input.UnsealKeyCount)

	persisted := false
	defer func() {
		if persisted {
			return
		}
		wipeInitializationMaterial(slots, credentials)
	}()

	for slotID := 1; slotID <= input.UnsealKeyCount; slotID++ {
		slot, credential, err := generateUnsealKeySlot(domain.UnsealSlotID(slotID))
		if err != nil {
			return nil, fmt.Errorf("generate unseal key slot %d: %w", slotID, err)
		}

		slots = append(slots, slot)
		credentials = append(credentials, credential)
	}

	if err := s.unsealRepository.CreateInitial(ctx, slots); err != nil {
		return nil, fmt.Errorf("persist initial unseal key slots: %w", err)
	}

	// The server-side key material now lives in durable storage.
	for i := range slots {
		clear(slots[i].ServerKeyMaterial)
	}

	persisted = true

	return &portin.InitializeVaultResult{Credentials: credentials}, nil
}

func (s *SealService) SubmitUnsealCredential(
	ctx context.Context,
	input portin.SubmitUnsealCredentialInput,
) (portin.SealStatus, error) {
	// The owner key is secret request material and must not survive this call.
	defer clear(input.OwnerSecret)

	if input.SlotID < 1 || len(input.OwnerSecret) != unsealKeySize || len(input.WrappedKey) == 0 {
		status, err := s.Status(ctx)
		if err != nil {
			return portin.SealStatus{}, err
		}
		return status, ErrInvalidCredential
	}

	count, err := s.unsealRepository.Count(ctx)
	if err != nil {
		return portin.SealStatus{}, fmt.Errorf("count unseal key slots: %w", err)
	}
	if count == 0 {
		return portin.SealStatus{}, ErrNotInitialized
	}

	// On process restart the in-memory store has no configuration.
	if err := s.secrets.ConfigureUnsealKeyCount(count); err != nil {
		return portin.SealStatus{}, fmt.Errorf("configure secret store: %w", err)
	}

	slot, err := s.unsealRepository.GetByID(ctx, input.SlotID)
	if err != nil {
		if errors.Is(err, portout.ErrUnsealKeySlotNotFound) {
			return s.statusFromCount(count), ErrInvalidCredential
		}
		return portin.SealStatus{}, fmt.Errorf("get unseal key slot: %w", err)
	}

	if slot.ID != input.SlotID || len(slot.ServerKeyMaterial) != unsealKeySize || len(slot.WrappedKey) == 0 {
		clear(slot.ServerKeyMaterial)
		return portin.SealStatus{}, ErrCorruptedUnsealSlot
	}
	defer clear(slot.ServerKeyMaterial)

	if subtle.ConstantTimeCompare(slot.WrappedKey, input.WrappedKey) != 1 {
		return s.statusFromCount(count), ErrInvalidCredential
	}

	wrappingKey := deriveUnsealWrappingKey(input.OwnerSecret, slot.ServerKeyMaterial)
	defer clear(wrappingKey[:])

	unsealKey, err := cryptox.Open(
		wrappingKey[:],
		cryptox.SealedPayload(slot.WrappedKey),
		unsealAAD(input.SlotID),
	)
	if err != nil {
		return s.statusFromCount(count), ErrInvalidCredential
	}

	// New takes ownership of unsealKey and wipes the Go slice before returning.
	secureUnsealKey, err := securemem.New(unsealKey)
	if err != nil {
		return portin.SealStatus{}, fmt.Errorf("protect recovered unseal key: %w", err)
	}

	progress, err := s.secrets.SubmitUnsealKey(int(input.SlotID), secureUnsealKey)
	status := portin.SealStatus{
		State:     sealState(progress.Unsealed),
		Submitted: progress.Submitted,
		Required:  progress.Required,
	}
	if err != nil {
		return status, err
	}

	return status, nil
}

func (s *SealService) Seal(ctx context.Context) error {
	count, err := s.unsealRepository.Count(ctx)
	if err != nil {
		return fmt.Errorf("count unseal key slots: %w", err)
	}
	if count == 0 {
		return ErrNotInitialized
	}

	// Idempotent and also clears a partial unseal attempt.
	s.secrets.Clear()
	return nil
}

func (s *SealService) Status(ctx context.Context) (portin.SealStatus, error) {
	count, err := s.unsealRepository.Count(ctx)
	if err != nil {
		return portin.SealStatus{}, fmt.Errorf("count unseal key slots: %w", err)
	}

	return s.statusFromCount(count), nil
}

func (s *SealService) statusFromCount(count int) portin.SealStatus {
	if count == 0 {
		return portin.SealStatus{State: domain.SealStateUninitialized}
	}

	progress := s.secrets.Progress()
	if s.secrets.IsUnsealed() {
		return portin.SealStatus{
			State:     domain.SealStateUnsealed,
			Submitted: count,
			Required:  count,
		}
	}

	return portin.SealStatus{
		State:     domain.SealStateSealed,
		Submitted: progress.Submitted,
		Required:  count,
	}
}

func sealState(unsealed bool) domain.SealState {
	if unsealed {
		return domain.SealStateUnsealed
	}
	return domain.SealStateSealed
}

func generateUnsealKeySlot(
	slotID domain.UnsealSlotID,
) (domain.UnsealKeySlot, domain.UnsealCredential, error) {
	ownerSecret, err := cryptox.RandomKey()
	if err != nil {
		return domain.UnsealKeySlot{}, domain.UnsealCredential{}, fmt.Errorf("generate owner key: %w", err)
	}

	serverKeyMaterial, err := cryptox.RandomKey()
	if err != nil {
		clear(ownerSecret)
		return domain.UnsealKeySlot{}, domain.UnsealCredential{}, fmt.Errorf("generate server key material: %w", err)
	}

	realUnsealKey, err := securemem.Random(unsealKeySize)
	if err != nil {
		clear(ownerSecret)
		clear(serverKeyMaterial)
		return domain.UnsealKeySlot{}, domain.UnsealCredential{}, fmt.Errorf("generate real unseal key: %w", err)
	}
	defer realUnsealKey.Destroy()

	wrappingKey := deriveUnsealWrappingKey(ownerSecret, serverKeyMaterial)
	defer clear(wrappingKey[:])

	var wrappedKey []byte
	if err := realUnsealKey.WithBytes(func(realKey []byte) error {
		var sealErr error
		wrappedKey, sealErr = cryptox.Seal(
			wrappingKey[:],
			realKey,
			unsealAAD(slotID),
		)
		return sealErr
	}); err != nil {
		clear(ownerSecret)
		clear(serverKeyMaterial)
		return domain.UnsealKeySlot{}, domain.UnsealCredential{}, fmt.Errorf("wrap real unseal key: %w", err)
	}

	credentialWrappedKey := bytes.Clone(wrappedKey)

	return domain.UnsealKeySlot{
			ID:                slotID,
			WrappedKey:        domain.WrappedUnsealKey(wrappedKey),
			ServerKeyMaterial: domain.ServerKeyMaterial(serverKeyMaterial),
		}, domain.UnsealCredential{
			Payload: domain.UnsealPayload{
				SlotID:     slotID,
				WrappedKey: domain.WrappedUnsealKey(credentialWrappedKey),
			},
			OwnerSecret: domain.OwnerSecret(ownerSecret),
		}, nil
}

// deriveUnsealWrappingKey computes SHA256(ownerSecret || serverKeyMaterial)
// without allocating a concatenated plaintext buffer.
func deriveUnsealWrappingKey(
	ownerSecret []byte,
	serverKeyMaterial []byte,
) [sha256.Size]byte {
	hasher := sha256.New()
	_, _ = hasher.Write(ownerSecret)
	_, _ = hasher.Write(serverKeyMaterial)

	var key [sha256.Size]byte
	hasher.Sum(key[:0])
	hasher.Reset()
	return key
}

func unsealAAD(slotID domain.UnsealSlotID) []byte {
	return fmt.Appendf(nil, "sigryx:unseal-key:v1:%d", slotID)
}

func wipeInitializationMaterial(
	slots []domain.UnsealKeySlot,
	credentials []domain.UnsealCredential,
) {
	for i := range slots {
		clear(slots[i].ServerKeyMaterial)
	}
	for i := range credentials {
		clear(credentials[i].OwnerSecret)
	}
}

var _ portin.SealUseCase = (*SealService)(nil)
