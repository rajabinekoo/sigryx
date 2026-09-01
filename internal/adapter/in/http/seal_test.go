package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
)

func TestInitializeVaultHTTP(t *testing.T) {
	fake := &fakeSealUseCase{
		initializeResult: &portin.InitializeVaultResult{
			Credentials: []domain.UnsealCredential{
				{
					Payload: domain.UnsealPayload{
						SlotID:     1,
						WrappedKey: []byte{1, 2, 3},
					},
					OwnerSecret: []byte{4, 5, 6},
				},
			},
		},
	}

	handler := New(Deps{Seal: fake})
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/vault/init",
		bytes.NewBufferString(`{"unseal_key_count":1}`),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if fake.initializeInput.UnsealKeyCount != 1 {
		t.Fatalf("expected unseal_key_count=1, got %d", fake.initializeInput.UnsealKeyCount)
	}

	var body struct {
		State       domain.SealState `json:"state"`
		Credentials []struct {
			SlotID        int    `json:"slot_id"`
			UnsealPayload string `json:"unseal_payload"`
			OwnerKey      string `json:"owner_key"`
		} `json:"credentials"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body.State != domain.SealStateSealed {
		t.Fatalf("expected SEALED, got %s", body.State)
	}
	if len(body.Credentials) != 1 {
		t.Fatalf("expected one credential, got %d", len(body.Credentials))
	}
	if body.Credentials[0].UnsealPayload != base64.RawURLEncoding.EncodeToString([]byte{1, 2, 3}) {
		t.Fatal("unexpected unseal payload encoding")
	}
	if body.Credentials[0].OwnerKey != base64.RawURLEncoding.EncodeToString([]byte{4, 5, 6}) {
		t.Fatal("unexpected owner key encoding")
	}
}

func TestSubmitUnsealHTTPDecodesCredential(t *testing.T) {
	fake := &fakeSealUseCase{
		submitStatus: portin.SealStatus{
			State:     domain.SealStateSealed,
			Submitted: 1,
			Required:  2,
		},
	}

	wrapped := []byte{1, 2, 3, 4}
	owner := bytes.Repeat([]byte{5}, 32)

	requestBody, err := json.Marshal(map[string]any{
		"slot_id":        2,
		"unseal_payload": base64.RawURLEncoding.EncodeToString(wrapped),
		"owner_key":      base64.RawURLEncoding.EncodeToString(owner),
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := New(Deps{Seal: fake})
	request := httptest.NewRequest(http.MethodPost, "/v1/vault/unseal", bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if fake.submitInput.SlotID != 2 {
		t.Fatalf("expected slot 2, got %d", fake.submitInput.SlotID)
	}
	if !bytes.Equal(fake.submitInput.WrappedKey, wrapped) {
		t.Fatal("wrapped key was not decoded correctly")
	}
	if !bytes.Equal(fake.submitInput.OwnerSecret, owner) {
		t.Fatal("owner key was not decoded correctly")
	}
}

type fakeSealUseCase struct {
	initializeInput  portin.InitializeVaultInput
	initializeResult *portin.InitializeVaultResult
	initializeErr    error

	submitInput  portin.SubmitUnsealCredentialInput
	submitStatus portin.SealStatus
	submitErr    error

	status    portin.SealStatus
	statusErr error
	sealErr   error
}

func (f *fakeSealUseCase) Initialize(
	_ context.Context,
	input portin.InitializeVaultInput,
) (*portin.InitializeVaultResult, error) {
	f.initializeInput = input
	return f.initializeResult, f.initializeErr
}

func (f *fakeSealUseCase) SubmitUnsealCredential(
	_ context.Context,
	input portin.SubmitUnsealCredentialInput,
) (portin.SealStatus, error) {
	f.submitInput = input
	return f.submitStatus, f.submitErr
}

func (f *fakeSealUseCase) Seal(context.Context) error {
	return f.sealErr
}

func (f *fakeSealUseCase) Status(context.Context) (portin.SealStatus, error) {
	return f.status, f.statusErr
}
