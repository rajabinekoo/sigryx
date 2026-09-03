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

func TestExportRecoveryHTTP(t *testing.T) {
	auth := &middlewareAuthStub{principal: domain.Principal{
		ID: "root-1", Kind: domain.PrincipalUser, RootAdmin: true,
	}}
	recovery := &fakeRecoveryUseCase{exportResult: &portin.ExportRecoveryResult{
		RecoveryKey: "rec_example",
		Backup:      "backup_example",
		KeyRoots:    2,
	}}

	handler := New(Deps{Auth: auth, Recovery: recovery})
	request := httptest.NewRequest(http.MethodPost, "/v1/recovery/export", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if !recovery.exportInput.Principal.RootAdmin {
		t.Fatal("root principal was not passed to recovery use case")
	}

	var body struct {
		RecoveryKey string `json:"recovery_key"`
		Backup      string `json:"backup"`
		KeyRoots    int    `json:"key_roots"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.RecoveryKey != "rec_example" || body.Backup != "backup_example" || body.KeyRoots != 2 {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestImportRecoveryHTTP(t *testing.T) {
	auth := &middlewareAuthStub{principal: domain.Principal{
		ID: "root-1", Kind: domain.PrincipalUser, RootAdmin: true,
	}}
	recovery := &fakeRecoveryUseCase{importResult: &portin.ImportRecoveryResult{KeyRoots: 3}}

	handler := New(Deps{Auth: auth, Recovery: recovery})
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/recovery/import",
		bytes.NewBufferString(`{"recovery_key":"rec_key","backup":"backup_blob"}`),
	)
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if recovery.importInput.RecoveryKey != "rec_key" || recovery.importInput.Backup != "backup_blob" {
		t.Fatalf("unexpected import input: %+v", recovery.importInput)
	}
	if !recovery.importInput.Principal.RootAdmin {
		t.Fatal("root principal was not passed to recovery use case")
	}
}

type fakeRecoveryUseCase struct {
	exportInput  portin.ExportRecoveryInput
	exportResult *portin.ExportRecoveryResult
	exportErr    error

	importInput  portin.ImportRecoveryInput
	importResult *portin.ImportRecoveryResult
	importErr    error
}

func (f *fakeRecoveryUseCase) Export(
	_ context.Context,
	input portin.ExportRecoveryInput,
) (*portin.ExportRecoveryResult, error) {
	f.exportInput = input
	return f.exportResult, f.exportErr
}

func (f *fakeRecoveryUseCase) Import(
	_ context.Context,
	input portin.ImportRecoveryInput,
) (*portin.ImportRecoveryResult, error) {
	f.importInput = input
	return f.importResult, f.importErr
}
