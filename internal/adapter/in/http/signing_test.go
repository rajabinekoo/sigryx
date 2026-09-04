package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
)

func TestSignTransactionHTTP(t *testing.T) {
	fake := &fakeSigningUseCase{
		transactionResult: &portin.SignTransactionResult{
			RawTransaction:  []byte{0x02, 0xaa},
			TransactionHash: bytes.Repeat([]byte{0x11}, 32),
			R:               bytes.Repeat([]byte{0x22}, 32),
			S:               bytes.Repeat([]byte{0x33}, 32),
			YParity:         1,
		},
	}

	response := serveJSON(t, New(Deps{Signing: fake}), http.MethodPost, "/v1/sign/transaction", `{
		"wallet_id":"wallet-1",
		"transaction":{
			"type":"EIP1559",
			"chain_id":1,
			"nonce":2,
			"gas_limit":65000,
			"max_priority_fee_per_gas":"1000000000",
			"max_fee_per_gas":"30000000000",
			"to":"0x3535353535353535353535353535353535353535",
			"value":"0",
			"data":"0xa9059cbb",
			"access_list":[{
				"address":"0x1111111111111111111111111111111111111111",
				"storage_keys":["0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]
			}]
		}
	}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	input := fake.transactionInput
	if input.WalletID != "wallet-1" {
		t.Fatalf("wallet id = %q", input.WalletID)
	}
	if input.Transaction.Type != domain.TransactionTypeEIP1559 || input.Transaction.ChainID != 1 || input.Transaction.Nonce != 2 {
		t.Fatalf("unexpected transaction: %+v", input.Transaction)
	}
	if input.Transaction.MaxPriorityFeePerGas != "1000000000" || input.Transaction.MaxFeePerGas != "30000000000" {
		t.Fatalf("unexpected fees: %+v", input.Transaction)
	}
	if len(input.Transaction.AccessList) != 1 || len(input.Transaction.AccessList[0].StorageKeys) != 1 {
		t.Fatalf("unexpected access list: %+v", input.Transaction.AccessList)
	}

	var body struct {
		RawTransaction  string `json:"raw_transaction"`
		TransactionHash string `json:"transaction_hash"`
		R               string `json:"r"`
		S               string `json:"s"`
		YParity         uint8  `json:"y_parity"`
	}
	decodeResponse(t, response, &body)

	if body.RawTransaction != "0x02aa" || body.YParity != 1 {
		t.Fatalf("unexpected response: %+v", body)
	}
	if body.TransactionHash != "0x"+strings.Repeat("11", 32) || body.R != "0x"+strings.Repeat("22", 32) || body.S != "0x"+strings.Repeat("33", 32) {
		t.Fatalf("unexpected signature response: %+v", body)
	}
}

func TestSignLegacyTransactionHTTP(t *testing.T) {
	fake := &fakeSigningUseCase{
		transactionResult: &portin.SignTransactionResult{
			RawTransaction:  []byte{0xf8, 0x6c},
			TransactionHash: bytes.Repeat([]byte{0x11}, 32),
			R:               bytes.Repeat([]byte{0x22}, 32),
			S:               bytes.Repeat([]byte{0x33}, 32),
			YParity:         0,
		},
	}

	response := serveJSON(t, New(Deps{Signing: fake}), http.MethodPost, "/v1/sign/transaction", `{
		"wallet_id":"wallet-legacy",
		"transaction":{
			"type":"LEGACY",
			"chain_id":1,
			"nonce":9,
			"gas_limit":21000,
			"gas_price":"20000000000",
			"to":"0x3535353535353535353535353535353535353535",
			"value":"1000000000000000000",
			"data":"0x"
		}
	}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}

	input := fake.transactionInput
	if input.WalletID != "wallet-legacy" || input.Transaction.Type != domain.TransactionTypeLegacy {
		t.Fatalf("unexpected input: %+v", input)
	}
	if input.Transaction.GasPrice != "20000000000" || input.Transaction.Value != "1000000000000000000" {
		t.Fatalf("unexpected legacy transaction: %+v", input.Transaction)
	}
	if input.Transaction.MaxPriorityFeePerGas != "" || input.Transaction.MaxFeePerGas != "" {
		t.Fatalf("unexpected EIP-1559 fields in legacy input: %+v", input.Transaction)
	}
}

func TestVerifyTransactionHTTP(t *testing.T) {
	fake := &fakeSigningUseCase{
		verifyTransactionResult: &portin.VerifyResult{Valid: true},
	}

	response := serveJSON(t, New(Deps{Signing: fake}), http.MethodPost, "/v1/verify/transaction", `{
		"wallet_id":"wallet-1",
		"raw_transaction":"0x02aabbcc"
	}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if fake.verifyTransactionInput.WalletID != "wallet-1" || !bytes.Equal(fake.verifyTransactionInput.RawTransaction, []byte{0x02, 0xaa, 0xbb, 0xcc}) {
		t.Fatalf("unexpected input: %+v", fake.verifyTransactionInput)
	}

	var body verifyResponse
	decodeResponse(t, response, &body)
	if !body.Valid || body.Digest != "" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestSignTypedDataHTTP(t *testing.T) {
	fake := &fakeSigningUseCase{
		typedDataResult: &portin.SignTypedDataResult{
			Signature: bytes.Repeat([]byte{0xaa}, 65),
			Digest:    bytes.Repeat([]byte{0xbb}, 32),
		},
	}

	response := serveJSON(t, New(Deps{Signing: fake}), http.MethodPost, "/v1/sign/typed-data", `{
		"wallet_id":"wallet-1",
		"typed_data":{
			"types":{
				"EIP712Domain":[{"name":"name","type":"string"}],
				"Transfer":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}]
			},
			"primaryType":"Transfer",
			"domain":{"name":"Sigryx Test"},
			"message":{"to":"0x3535353535353535353535353535353535353535","amount":"1000000"}
		}
	}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if fake.typedDataInput.WalletID != "wallet-1" {
		t.Fatalf("wallet id = %q", fake.typedDataInput.WalletID)
	}
	assertTypedData(t, fake.typedDataInput.TypedData, "Transfer")

	var body struct {
		Signature string `json:"signature"`
		Digest    string `json:"digest"`
	}
	decodeResponse(t, response, &body)
	if body.Signature != "0x"+strings.Repeat("aa", 65) || body.Digest != "0x"+strings.Repeat("bb", 32) {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestVerifyTypedDataHTTP(t *testing.T) {
	signature := bytes.Repeat([]byte{0xcc}, 65)
	fake := &fakeSigningUseCase{
		verifyTypedDataResult: &portin.VerifyResult{Valid: true, Digest: bytes.Repeat([]byte{0xdd}, 32)},
	}

	response := serveJSON(t, New(Deps{Signing: fake}), http.MethodPost, "/v1/verify/typed-data", `{
		"wallet_id":"wallet-1",
		"typed_data":{
			"types":{
				"EIP712Domain":[{"name":"name","type":"string"}],
				"Transfer":[{"name":"amount","type":"uint256"}]
			},
			"primaryType":"Transfer",
			"domain":{"name":"Sigryx Test"},
			"message":{"amount":"1000000"}
		},
		"signature":"0x`+hex.EncodeToString(signature)+`"
	}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if fake.verifyTypedDataInput.WalletID != "wallet-1" || !bytes.Equal(fake.verifyTypedDataInput.Signature, signature) {
		t.Fatalf("unexpected input: %+v", fake.verifyTypedDataInput)
	}
	assertTypedData(t, fake.verifyTypedDataInput.TypedData, "Transfer")

	var body verifyResponse
	decodeResponse(t, response, &body)
	if !body.Valid || body.Digest != "0x"+strings.Repeat("dd", 32) {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestSignGenericJSONHTTP(t *testing.T) {
	fake := &fakeSigningUseCase{
		dataResult: &portin.SignDataResult{
			Signature: bytes.Repeat([]byte{0xaa}, 64),
			Digest:    bytes.Repeat([]byte{0xbb}, 32),
		},
	}

	response := serveJSON(t, New(Deps{Signing: fake}), http.MethodPost, "/v1/sign/data", `{
		"wallet_id":"wallet-1",
		"context":"ledger:journal-entry:v1",
		"format":"JSON",
		"payload":{"sequence":7,"amount":"10"}
	}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if fake.dataInput.WalletID != "wallet-1" || fake.dataInput.Context != "ledger:journal-entry:v1" || fake.dataInput.Format != domain.DataFormatJSON {
		t.Fatalf("unexpected input: %+v", fake.dataInput)
	}
	assertJSONPayload(t, fake.dataInput.Payload, "amount", "10")

	var body struct {
		Signature string `json:"signature"`
		Digest    string `json:"digest"`
	}
	decodeResponse(t, response, &body)
	if body.Signature != "0x"+strings.Repeat("aa", 64) || body.Digest != "0x"+strings.Repeat("bb", 32) {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestVerifyGenericJSONHTTP(t *testing.T) {
	signature := bytes.Repeat([]byte{0xcc}, 64)
	fake := &fakeSigningUseCase{
		verifyDataResult: &portin.VerifyResult{Valid: true, Digest: bytes.Repeat([]byte{0xdd}, 32)},
	}

	response := serveJSON(t, New(Deps{Signing: fake}), http.MethodPost, "/v1/verify/data", `{
		"wallet_id":"wallet-1",
		"context":"ledger:journal-entry:v1",
		"format":"JSON",
		"payload":{"sequence":7,"amount":"10"},
		"signature":"0x`+hex.EncodeToString(signature)+`"
	}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if fake.verifyDataInput.WalletID != "wallet-1" || fake.verifyDataInput.Context != "ledger:journal-entry:v1" || fake.verifyDataInput.Format != domain.DataFormatJSON {
		t.Fatalf("unexpected input: %+v", fake.verifyDataInput)
	}
	if !bytes.Equal(fake.verifyDataInput.Signature, signature) {
		t.Fatal("signature was not decoded correctly")
	}
	assertJSONPayload(t, fake.verifyDataInput.Payload, "amount", "10")

	var body verifyResponse
	decodeResponse(t, response, &body)
	if !body.Valid || body.Digest != "0x"+strings.Repeat("dd", 32) {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestSignGenericRawHTTP(t *testing.T) {
	fake := &fakeSigningUseCase{
		dataResult: &portin.SignDataResult{
			Signature: bytes.Repeat([]byte{0xaa}, 64),
			Digest:    bytes.Repeat([]byte{0xbb}, 32),
		},
	}
	payload := base64.RawURLEncoding.EncodeToString([]byte("journal-entry"))

	response := serveJSON(t, New(Deps{Signing: fake}), http.MethodPost, "/v1/sign/data", `{
		"wallet_id":"wallet-1",
		"context":"ledger:journal-entry:v1",
		"format":"RAW",
		"payload":"`+payload+`"
	}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if fake.dataInput.Format != domain.DataFormatRaw || string(fake.dataInput.Payload) != "journal-entry" {
		t.Fatalf("unexpected input: %+v", fake.dataInput)
	}
}

func TestVerifyGenericRawHTTP(t *testing.T) {
	signature := bytes.Repeat([]byte{0xcc}, 64)
	payload := base64.RawURLEncoding.EncodeToString([]byte{0x00, 0x01, 0xff})
	fake := &fakeSigningUseCase{
		verifyDataResult: &portin.VerifyResult{Valid: false, Digest: bytes.Repeat([]byte{0xdd}, 32)},
	}

	response := serveJSON(t, New(Deps{Signing: fake}), http.MethodPost, "/v1/verify/data", `{
		"wallet_id":"wallet-1",
		"context":"binary:test:v1",
		"format":"RAW",
		"payload":"`+payload+`",
		"signature":"0x`+hex.EncodeToString(signature)+`"
	}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if !bytes.Equal(fake.verifyDataInput.Payload, []byte{0x00, 0x01, 0xff}) || !bytes.Equal(fake.verifyDataInput.Signature, signature) {
		t.Fatalf("unexpected input: %+v", fake.verifyDataInput)
	}

	var body verifyResponse
	decodeResponse(t, response, &body)
	if body.Valid || body.Digest != "0x"+strings.Repeat("dd", 32) {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestSignIntegrityHTTPWithNestedFields(t *testing.T) {
	fake := &fakeSigningUseCase{integrityResult: &portin.SignIntegrityResult{
		Signature: bytes.Repeat([]byte{0xaa}, 64), Digest: bytes.Repeat([]byte{0xbb}, 32), Reused: true,
	}}
	response := serveJSON(t, New(Deps{Signing: fake}), http.MethodPost, "/v1/sign/integrity", `{
		"wallet_id":"wallet-1",
		"context":"ledger:journal-entry:v1",
		"object_id":"journal-100",
		"payload":{"id":"journal-100","lines":[{"amount":"10"}],"meta":{"note":"mutable"}},
		"integrity_fields":["/id","/lines/0/amount"]
	}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if fake.integrityInput.ObjectID != "journal-100" || len(fake.integrityInput.IntegrityFields) != 2 {
		t.Fatalf("unexpected input: %+v", fake.integrityInput)
	}
	var body struct {
		Signature string `json:"signature"`
		Digest    string `json:"digest"`
		Reused    bool   `json:"reused"`
	}
	decodeResponse(t, response, &body)
	if !body.Reused || body.Signature != "0x"+strings.Repeat("aa", 64) || body.Digest != "0x"+strings.Repeat("bb", 32) {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestVerifyIntegrityHTTP(t *testing.T) {
	signature := bytes.Repeat([]byte{0xcc}, 64)
	fake := &fakeSigningUseCase{verifyIntegrityResult: &portin.VerifyIntegrityResult{
		Valid: false, SignatureValid: false, RecordMatch: false,
		Digest: bytes.Repeat([]byte{0xdd}, 32), Reason: "INTEGRITY_VALUE_MISMATCH",
	}}
	response := serveJSON(t, New(Deps{Signing: fake}), http.MethodPost, "/v1/verify/integrity", `{
		"wallet_id":"wallet-1",
		"context":"ledger:journal-entry:v1",
		"object_id":"journal-100",
		"payload":{"id":"journal-100","customer":{"amount":"20"}},
		"integrity_fields":["/id","/customer/amount"],
		"signature":"0x`+hex.EncodeToString(signature)+`"
	}`)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if !bytes.Equal(fake.verifyIntegrityInput.Signature, signature) || fake.verifyIntegrityInput.ObjectID != "journal-100" {
		t.Fatalf("unexpected input: %+v", fake.verifyIntegrityInput)
	}
	var body struct {
		Valid          bool   `json:"valid"`
		SignatureValid bool   `json:"signature_valid"`
		RecordMatch    bool   `json:"record_match"`
		Digest         string `json:"digest"`
		Reason         string `json:"reason"`
	}
	decodeResponse(t, response, &body)
	if body.Valid || body.RecordMatch || body.Reason != "INTEGRITY_VALUE_MISMATCH" || body.Digest != "0x"+strings.Repeat("dd", 32) {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestSigningHTTPRejectsInvalidEncodedInputs(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{
			name: "transaction raw hex",
			path: "/v1/verify/transaction",
			body: `{"wallet_id":"wallet-1","raw_transaction":"0xabc"}`,
		},
		{
			name: "typed data signature hex",
			path: "/v1/verify/typed-data",
			body: `{"wallet_id":"wallet-1","typed_data":{},"signature":"0xzz"}`,
		},
		{
			name: "raw base64url payload",
			path: "/v1/sign/data",
			body: `{"wallet_id":"wallet-1","context":"test:v1","format":"RAW","payload":"***"}`,
		},
		{
			name: "unknown generic format",
			path: "/v1/sign/data",
			body: `{"wallet_id":"wallet-1","context":"test:v1","format":"TEXT","payload":"abc"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := serveJSON(t, New(Deps{Signing: &fakeSigningUseCase{}}), http.MethodPost, tt.path, tt.body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

type verifyResponse struct {
	Valid  bool   `json:"valid"`
	Digest string `json:"digest"`
}

type fakeSigningUseCase struct {
	transactionInput        portin.SignTransactionInput
	transactionResult       *portin.SignTransactionResult
	verifyTransactionInput  portin.VerifyTransactionInput
	verifyTransactionResult *portin.VerifyResult
	typedDataInput          portin.SignTypedDataInput
	typedDataResult         *portin.SignTypedDataResult
	verifyTypedDataInput    portin.VerifyTypedDataInput
	verifyTypedDataResult   *portin.VerifyResult
	dataInput               portin.SignDataInput
	dataResult              *portin.SignDataResult
	verifyDataInput         portin.VerifyDataInput
	verifyDataResult        *portin.VerifyResult
	integrityInput          portin.SignIntegrityInput
	integrityResult         *portin.SignIntegrityResult
	verifyIntegrityInput    portin.VerifyIntegrityInput
	verifyIntegrityResult   *portin.VerifyIntegrityResult
}

func (f *fakeSigningUseCase) SignTransaction(_ context.Context, input portin.SignTransactionInput) (*portin.SignTransactionResult, error) {
	f.transactionInput = input
	return f.transactionResult, nil
}

func (f *fakeSigningUseCase) VerifyTransaction(_ context.Context, input portin.VerifyTransactionInput) (*portin.VerifyResult, error) {
	f.verifyTransactionInput = input
	return f.verifyTransactionResult, nil
}

func (f *fakeSigningUseCase) SignTypedData(_ context.Context, input portin.SignTypedDataInput) (*portin.SignTypedDataResult, error) {
	f.typedDataInput = input
	return f.typedDataResult, nil
}

func (f *fakeSigningUseCase) VerifyTypedData(_ context.Context, input portin.VerifyTypedDataInput) (*portin.VerifyResult, error) {
	f.verifyTypedDataInput = input
	return f.verifyTypedDataResult, nil
}

func (f *fakeSigningUseCase) SignData(_ context.Context, input portin.SignDataInput) (*portin.SignDataResult, error) {
	f.dataInput = input
	return f.dataResult, nil
}

func (f *fakeSigningUseCase) VerifyData(_ context.Context, input portin.VerifyDataInput) (*portin.VerifyResult, error) {
	f.verifyDataInput = input
	return f.verifyDataResult, nil
}

func (f *fakeSigningUseCase) SignIntegrity(_ context.Context, input portin.SignIntegrityInput) (*portin.SignIntegrityResult, error) {
	f.integrityInput = input
	return f.integrityResult, nil
}

func (f *fakeSigningUseCase) VerifyIntegrity(_ context.Context, input portin.VerifyIntegrityInput) (*portin.VerifyIntegrityResult, error) {
	f.verifyIntegrityInput = input
	return f.verifyIntegrityResult, nil
}

func serveJSON(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatal(err)
	}
}

func assertTypedData(t *testing.T, raw []byte, primaryType string) {
	t.Helper()
	var value struct {
		PrimaryType string `json:"primaryType"`
	}
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if value.PrimaryType != primaryType {
		t.Fatalf("primary type = %q", value.PrimaryType)
	}
}

func assertJSONPayload(t *testing.T, raw []byte, key string, expected any) {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	if value[key] != expected {
		t.Fatalf("%s = %#v, expected %#v", key, value[key], expected)
	}
}
