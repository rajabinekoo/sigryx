package service

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/pkg/secretstore"
)

func TestSealServiceInitializeAndUnseal(t *testing.T) {
	repo := newMemoryUnsealRepository()
	secrets := secretstore.New()
	defer secrets.Clear()

	svc := NewSealService(repo, secrets, 10)

	result, err := svc.Initialize(context.Background(), portin.InitializeVaultInput{
		UnsealKeyCount: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Credentials) != 3 {
		t.Fatalf("expected 3 credentials, got %d", len(result.Credentials))
	}
	if secrets.IsUnsealed() {
		t.Fatal("initialization must leave the vault sealed")
	}

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertSealStatus(t, status, domain.SealStateSealed, 0, 3)

	// Submit deliberately out of slot order. SecretStore must derive the final
	// vault key by stable slot order, not submission order.
	for i, credentialIndex := range []int{2, 0, 1} {
		credential := result.Credentials[credentialIndex]
		status, err = svc.SubmitUnsealCredential(
			context.Background(),
			portin.SubmitUnsealCredentialInput{
				SlotID:      credential.Payload.SlotID,
				WrappedKey:  bytes.Clone(credential.Payload.WrappedKey),
				OwnerSecret: bytes.Clone(credential.OwnerSecret),
			},
		)
		if err != nil {
			t.Fatal(err)
		}

		if i < 2 && status.State != domain.SealStateSealed {
			t.Fatalf("expected SEALED before final credential, got %s", status.State)
		}
	}

	assertSealStatus(t, status, domain.SealStateUnsealed, 3, 3)
	if !secrets.IsUnsealed() {
		t.Fatal("vault should be unsealed after all credentials")
	}
}

func TestSealServiceRejectsInvalidOwnerKey(t *testing.T) {
	repo := newMemoryUnsealRepository()
	secrets := secretstore.New()
	defer secrets.Clear()

	svc := NewSealService(repo, secrets, 10)
	result, err := svc.Initialize(context.Background(), portin.InitializeVaultInput{
		UnsealKeyCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	credential := result.Credentials[0]
	wrongOwnerKey := bytes.Repeat([]byte{0x42}, unsealKeySize)

	status, err := svc.SubmitUnsealCredential(
		context.Background(),
		portin.SubmitUnsealCredentialInput{
			SlotID:      credential.Payload.SlotID,
			WrappedKey:  bytes.Clone(credential.Payload.WrappedKey),
			OwnerSecret: wrongOwnerKey,
		},
	)
	if !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("expected ErrInvalidCredential, got %v", err)
	}
	assertSealStatus(t, status, domain.SealStateSealed, 0, 1)
	if !bytes.Equal(wrongOwnerKey, make([]byte, len(wrongOwnerKey))) {
		t.Fatal("owner key must be wiped before SubmitUnsealCredential returns")
	}
}

func TestSealServiceRejectsInitializationTwice(t *testing.T) {
	repo := newMemoryUnsealRepository()
	secrets := secretstore.New()
	defer secrets.Clear()

	svc := NewSealService(repo, secrets, 10)
	if _, err := svc.Initialize(context.Background(), portin.InitializeVaultInput{UnsealKeyCount: 1}); err != nil {
		t.Fatal(err)
	}

	_, err := svc.Initialize(context.Background(), portin.InitializeVaultInput{UnsealKeyCount: 1})
	if !errors.Is(err, portout.ErrAlreadyInitialized) {
		t.Fatalf("expected ErrAlreadyInitialized, got %v", err)
	}
}

func TestSealServiceHonorsMaxUnsealKeyCount(t *testing.T) {
	svc := NewSealService(newMemoryUnsealRepository(), secretstore.New(), 3)

	_, err := svc.Initialize(context.Background(), portin.InitializeVaultInput{UnsealKeyCount: 4})
	if !errors.Is(err, ErrInvalidUnsealKeyCount) {
		t.Fatalf("expected ErrInvalidUnsealKeyCount, got %v", err)
	}
}

func TestSealServiceSealClearsRuntimeSecrets(t *testing.T) {
	repo := newMemoryUnsealRepository()
	secrets := secretstore.New()
	defer secrets.Clear()

	svc := NewSealService(repo, secrets, 10)
	result, err := svc.Initialize(context.Background(), portin.InitializeVaultInput{UnsealKeyCount: 1})
	if err != nil {
		t.Fatal(err)
	}

	credential := result.Credentials[0]
	if _, err := svc.SubmitUnsealCredential(
		context.Background(),
		portin.SubmitUnsealCredentialInput{
			SlotID:      credential.Payload.SlotID,
			WrappedKey:  bytes.Clone(credential.Payload.WrappedKey),
			OwnerSecret: bytes.Clone(credential.OwnerSecret),
		},
	); err != nil {
		t.Fatal(err)
	}

	if err := svc.Seal(context.Background()); err != nil {
		t.Fatal(err)
	}
	if secrets.IsUnsealed() {
		t.Fatal("vault should be sealed")
	}

	status, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertSealStatus(t, status, domain.SealStateSealed, 0, 1)
}

func assertSealStatus(
	t *testing.T,
	status portin.SealStatus,
	state domain.SealState,
	submitted int,
	required int,
) {
	t.Helper()
	if status.State != state || status.Submitted != submitted || status.Required != required {
		t.Fatalf(
			"unexpected status: state=%s submitted=%d required=%d",
			status.State,
			status.Submitted,
			status.Required,
		)
	}
}

type memoryUnsealRepository struct {
	mu    sync.Mutex
	slots map[domain.UnsealSlotID]domain.UnsealKeySlot
}

func newMemoryUnsealRepository() *memoryUnsealRepository {
	return &memoryUnsealRepository{slots: make(map[domain.UnsealSlotID]domain.UnsealKeySlot)}
}

func (r *memoryUnsealRepository) Count(context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.slots), nil
}

func (r *memoryUnsealRepository) CreateInitial(
	_ context.Context,
	slots []domain.UnsealKeySlot,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.slots) != 0 {
		return portout.ErrAlreadyInitialized
	}

	for _, slot := range slots {
		if _, exists := r.slots[slot.ID]; exists {
			return portout.ErrAlreadyInitialized
		}
	}

	for _, slot := range slots {
		r.slots[slot.ID] = domain.UnsealKeySlot{
			ID:                slot.ID,
			WrappedKey:        bytes.Clone(slot.WrappedKey),
			ServerKeyMaterial: bytes.Clone(slot.ServerKeyMaterial),
		}
	}
	return nil
}

func (r *memoryUnsealRepository) GetByID(
	_ context.Context,
	id domain.UnsealSlotID,
) (*domain.UnsealKeySlot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	slot, ok := r.slots[id]
	if !ok {
		return nil, portout.ErrUnsealKeySlotNotFound
	}

	return &domain.UnsealKeySlot{
		ID:                slot.ID,
		WrappedKey:        bytes.Clone(slot.WrappedKey),
		ServerKeyMaterial: bytes.Clone(slot.ServerKeyMaterial),
	}, nil
}
