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
)

func TestCreateWalletHTTP(t *testing.T) {
	fake := &fakeWalletUseCase{
		result: &portin.WalletResult{
			ID:             "wallet-1",
			KeyRootID:      "01900000-0000-7000-8000-000000000001",
			UserID:         "user-42",
			WalletType:     domain.WalletTypeEthereum,
			Adapter:        "evm",
			DerivationPath: "m/44'/60'/0'/0/5",
			PublicKey:      []byte{0x04, 0x01, 0x02},
			Address:        "0xabc",
		},
	}

	handler := New(Deps{Wallets: fake})
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/wallets",
		bytes.NewBufferString(`{"key_root_id":"01900000-0000-7000-8000-000000000001","user_id":"user-42","wallet_type":"ETHEREUM"}`),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if fake.input.UserID != "user-42" {
		t.Fatalf("user ID = %q, want user-42", fake.input.UserID)
	}
	if fake.input.KeyRootID != fake.result.KeyRootID {
		t.Fatalf("key root ID = %q, want %q", fake.input.KeyRootID, fake.result.KeyRootID)
	}

	var body struct {
		ID             string `json:"id"`
		UserID         string `json:"user_id"`
		PublicKey      string `json:"public_key"`
		Address        string `json:"address"`
		DerivationPath string `json:"derivation_path"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ID != "wallet-1" || body.UserID != "user-42" {
		t.Fatalf("unexpected body: %+v", body)
	}
	if body.PublicKey != "0x040102" {
		t.Fatalf("public key = %q, want 0x040102", body.PublicKey)
	}
}

type fakeWalletUseCase struct {
	input  portin.CreateWalletInput
	result *portin.WalletResult
	err    error
}

func (f *fakeWalletUseCase) Create(
	_ context.Context,
	input portin.CreateWalletInput,
) (*portin.WalletResult, error) {
	f.input = input
	return f.result, f.err
}
