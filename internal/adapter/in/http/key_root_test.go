package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	"github.com/rajabinekoo/sigryx/pkg/secretstore"
)

func TestCreateKeyRootHTTP(t *testing.T) {
	fake := &fakeKeyRootUseCase{
		result: &portin.CreateKeyRootResult{
			ID:               "01900000-0000-7000-8000-000000000001",
			WalletType:       domain.WalletTypeEthereum,
			DerivationScheme: domain.DerivationSchemeBIP32Secp256k1,
		},
	}

	handler := New(Deps{KeyRoots: fake})
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/key-roots",
		bytes.NewBufferString(`{"wallet_type":"ETHEREUM"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if fake.input.WalletType != domain.WalletTypeEthereum {
		t.Fatalf("wallet type = %q, want ETHEREUM", fake.input.WalletType)
	}

	var body struct {
		ID               string                  `json:"id"`
		WalletType       domain.WalletType       `json:"wallet_type"`
		DerivationScheme domain.DerivationScheme `json:"derivation_scheme"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	if body.ID != fake.result.ID {
		t.Fatalf("ID = %q, want %q", body.ID, fake.result.ID)
	}
	if body.WalletType != domain.WalletTypeEthereum {
		t.Fatalf("wallet type = %q, want ETHEREUM", body.WalletType)
	}
	if body.DerivationScheme != domain.DerivationSchemeBIP32Secp256k1 {
		t.Fatalf("unexpected derivation scheme: %q", body.DerivationScheme)
	}
}

func TestCreateKeyRootHTTPRequiresUnsealedVault(t *testing.T) {
	fake := &fakeKeyRootUseCase{err: secretstore.ErrVaultSealed}

	handler := New(Deps{KeyRoots: fake})
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/key-roots",
		bytes.NewBufferString(`{"wallet_type":"ETHEREUM"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", response.Code, response.Body.String())
	}
}

type fakeKeyRootUseCase struct {
	input  portin.CreateKeyRootInput
	result *portin.CreateKeyRootResult
	err    error
}

func (f *fakeKeyRootUseCase) GetAll(context.Context) ([]portin.KeyRootResult, error) {
	return nil, f.err
}

func (f *fakeKeyRootUseCase) Create(
	_ context.Context,
	input portin.CreateKeyRootInput,
) (*portin.CreateKeyRootResult, error) {
	f.input = input
	return f.result, f.err
}
