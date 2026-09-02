package http

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
)

type transactionBody struct {
	Type                 domain.TransactionType `json:"type" doc:"Transaction envelope type. Supported values: LEGACY and EIP1559."`
	ChainID              uint64                 `json:"chain_id" doc:"EVM chain ID used for replay protection, for example 1 for Ethereum mainnet."`
	Nonce                uint64                 `json:"nonce" doc:"Sender account transaction nonce."`
	GasLimit             uint64                 `json:"gas_limit" doc:"Maximum gas units the transaction may consume."`
	GasPrice             string                 `json:"gas_price,omitempty" doc:"LEGACY only. Gas price in wei as a decimal string or 0x hexadecimal quantity."`
	MaxPriorityFeePerGas string                 `json:"max_priority_fee_per_gas,omitempty" doc:"EIP1559 only. Priority fee per gas in wei as a decimal string or 0x hexadecimal quantity."`
	MaxFeePerGas         string                 `json:"max_fee_per_gas,omitempty" doc:"EIP1559 only. Maximum total fee per gas in wei as a decimal string or 0x hexadecimal quantity."`
	To                   string                 `json:"to,omitempty" doc:"Destination 20-byte EVM address in hex. Leave empty only for contract creation."`
	Value                string                 `json:"value,omitempty" doc:"Native-asset value in wei as a decimal string or 0x hexadecimal quantity. Empty means zero."`
	Data                 string                 `json:"data,omitempty" doc:"Hex-encoded transaction calldata. Use 0x or empty for no calldata."`
	AccessList           []accessListBody       `json:"access_list,omitempty" doc:"Optional EIP-2930 access list carried by an EIP1559 transaction."`
}

type accessListBody struct {
	Address     string   `json:"address" doc:"20-byte EVM contract/account address in hex."`
	StorageKeys []string `json:"storage_keys" doc:"Storage slots accessed at this address. Every value must be exactly 32 bytes in hex."`
}

type signTransactionInput struct {
	Body struct {
		WalletID    string          `json:"wallet_id" doc:"Sigryx wallet ID whose derived private key signs the transaction."`
		Transaction transactionBody `json:"transaction" doc:"Unsigned Ethereum transaction fields."`
	}
}

type signTransactionOutput struct {
	Body struct {
		RawTransaction  string `json:"raw_transaction" doc:"0x-prefixed serialized signed transaction. Send this value to eth_sendRawTransaction."`
		TransactionHash string `json:"transaction_hash" doc:"0x-prefixed Keccak-256 hash of raw_transaction."`
		R               string `json:"r" doc:"0x-prefixed 32-byte ECDSA r component."`
		S               string `json:"s" doc:"0x-prefixed 32-byte low-S ECDSA s component."`
		YParity         uint8  `json:"y_parity" doc:"Ethereum signature recovery/y-parity bit. Always 0 or 1."`
	}
}

type verifyTransactionInput struct {
	Body struct {
		WalletID       string `json:"wallet_id" doc:"Sigryx wallet ID whose persisted public key is used for verification."`
		RawTransaction string `json:"raw_transaction" doc:"0x-prefixed serialized signed Ethereum transaction."`
	}
}

type verifyOutput struct {
	Body struct {
		Valid  bool   `json:"valid" doc:"True when the signature belongs to the requested wallet and the signed payload is structurally valid."`
		Digest string `json:"digest,omitempty" doc:"0x-prefixed 32-byte digest that was verified. Returned for EIP-712 and generic data verification."`
	}
}

type signTypedDataInput struct {
	Body struct {
		WalletID  string          `json:"wallet_id" doc:"Sigryx wallet ID whose derived private key signs the EIP-712 payload."`
		TypedData json.RawMessage `json:"typed_data" doc:"Complete EIP-712 object containing types, primaryType, domain, and message."`
	}
}

type signTypedDataOutput struct {
	Body struct {
		Signature string `json:"signature" doc:"0x-prefixed 65-byte Ethereum ECDSA signature encoded as r || s || v, where v is 27 or 28."`
		Digest    string `json:"digest" doc:"0x-prefixed 32-byte EIP-712 Keccak-256 digest that was signed."`
	}
}

type verifyTypedDataInput struct {
	Body struct {
		WalletID  string          `json:"wallet_id" doc:"Sigryx wallet ID whose persisted public key is used for EIP-712 verification."`
		TypedData json.RawMessage `json:"typed_data" doc:"Complete EIP-712 object containing types, primaryType, domain, and message."`
		Signature string          `json:"signature" doc:"0x-prefixed 65-byte EIP-712 Ethereum signature encoded as r || s || v."`
	}
}

type signDataInput struct {
	Body struct {
		WalletID string            `json:"wallet_id" doc:"Sigryx wallet ID used for signing."`
		Context  string            `json:"context" doc:"Required application-level domain separator, for example ledger:journal-entry:v1. The same context must be supplied during verification."`
		Format   domain.DataFormat `json:"format" doc:"Payload interpretation. JSON canonicalizes JSON before hashing; RAW signs the decoded bytes as-is."`
		Payload  json.RawMessage   `json:"payload" doc:"For JSON: any valid JSON value. For RAW: a base64url-encoded string containing the exact bytes to sign."`
	}
}

type signDataOutput struct {
	Body struct {
		Signature string `json:"signature" doc:"0x-prefixed 64-byte compact secp256k1 signature encoded as r || s."`
		Digest    string `json:"digest" doc:"0x-prefixed 32-byte SHA-256 digest produced by Sigryx framing, context, format, and payload."`
	}
}

type verifyDataInput struct {
	Body struct {
		WalletID  string            `json:"wallet_id" doc:"Sigryx wallet ID whose persisted public key is used for verification."`
		Context   string            `json:"context" doc:"Must exactly match the context used when the signature was created."`
		Format    domain.DataFormat `json:"format" doc:"Must match the original signing format: JSON or RAW."`
		Payload   json.RawMessage   `json:"payload" doc:"Original payload. JSON is canonicalized; RAW must be the original bytes encoded as base64url."`
		Signature string            `json:"signature" doc:"0x-prefixed 64-byte compact secp256k1 signature returned by /v1/sign/data."`
	}
}

func registerSigningRoutes(api huma.API, signing portin.SigningUseCase) {
	huma.Register(api, huma.Operation{
		OperationID: "sign_transaction", Method: http.MethodPost, Path: "/v1/sign/transaction",
		Summary: "Sign an Ethereum transaction", Tags: []string{"signing"},
	}, func(ctx context.Context, in *signTransactionInput) (*signTransactionOutput, error) {
		result, err := signing.SignTransaction(ctx, portin.SignTransactionInput{
			WalletID:    in.Body.WalletID,
			Transaction: transactionFromHTTP(in.Body.Transaction),
		})
		if err != nil {
			return nil, translate(err)
		}
		out := &signTransactionOutput{}
		out.Body.RawTransaction = hex0x(result.RawTransaction)
		out.Body.TransactionHash = hex0x(result.TransactionHash)
		out.Body.R = hex0x(result.R)
		out.Body.S = hex0x(result.S)
		out.Body.YParity = result.YParity
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "verify_transaction", Method: http.MethodPost, Path: "/v1/verify/transaction",
		Summary: "Verify a signed Ethereum transaction", Tags: []string{"signing"},
	}, func(ctx context.Context, in *verifyTransactionInput) (*verifyOutput, error) {
		raw, err := parseHex(in.Body.RawTransaction)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid raw_transaction")
		}
		result, err := signing.VerifyTransaction(ctx, portin.VerifyTransactionInput{WalletID: in.Body.WalletID, RawTransaction: raw})
		if err != nil {
			return nil, translate(err)
		}
		return verifyHTTPResult(result), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "sign_typed_data", Method: http.MethodPost, Path: "/v1/sign/typed-data",
		Summary: "Sign EIP-712 typed data", Tags: []string{"signing"},
	}, func(ctx context.Context, in *signTypedDataInput) (*signTypedDataOutput, error) {
		result, err := signing.SignTypedData(ctx, portin.SignTypedDataInput{WalletID: in.Body.WalletID, TypedData: in.Body.TypedData})
		if err != nil {
			return nil, translate(err)
		}
		out := &signTypedDataOutput{}
		out.Body.Signature = hex0x(result.Signature)
		out.Body.Digest = hex0x(result.Digest)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "verify_typed_data", Method: http.MethodPost, Path: "/v1/verify/typed-data",
		Summary: "Verify an EIP-712 signature", Tags: []string{"signing"},
	}, func(ctx context.Context, in *verifyTypedDataInput) (*verifyOutput, error) {
		signature, err := parseHex(in.Body.Signature)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid signature")
		}
		result, err := signing.VerifyTypedData(ctx, portin.VerifyTypedDataInput{WalletID: in.Body.WalletID, TypedData: in.Body.TypedData, Signature: signature})
		if err != nil {
			return nil, translate(err)
		}
		return verifyHTTPResult(result), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "sign_data", Method: http.MethodPost, Path: "/v1/sign/data",
		Summary: "Sign generic raw or JSON data", Tags: []string{"signing"},
	}, func(ctx context.Context, in *signDataInput) (*signDataOutput, error) {
		payload, err := signingPayload(in.Body.Format, in.Body.Payload)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		result, err := signing.SignData(ctx, portin.SignDataInput{WalletID: in.Body.WalletID, Context: in.Body.Context, Format: in.Body.Format, Payload: payload})
		if err != nil {
			return nil, translate(err)
		}
		out := &signDataOutput{}
		out.Body.Signature = hex0x(result.Signature)
		out.Body.Digest = hex0x(result.Digest)
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "verify_data", Method: http.MethodPost, Path: "/v1/verify/data",
		Summary: "Verify generic raw or JSON data", Tags: []string{"signing"},
	}, func(ctx context.Context, in *verifyDataInput) (*verifyOutput, error) {
		payload, err := signingPayload(in.Body.Format, in.Body.Payload)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		signature, err := parseHex(in.Body.Signature)
		if err != nil {
			return nil, huma.Error400BadRequest("invalid signature")
		}
		result, err := signing.VerifyData(ctx, portin.VerifyDataInput{WalletID: in.Body.WalletID, Context: in.Body.Context, Format: in.Body.Format, Payload: payload, Signature: signature})
		if err != nil {
			return nil, translate(err)
		}
		return verifyHTTPResult(result), nil
	})
}

func transactionFromHTTP(value transactionBody) domain.EthereumTransaction {
	accessList := make([]domain.AccessListEntry, len(value.AccessList))
	for i, item := range value.AccessList {
		accessList[i] = domain.AccessListEntry{Address: item.Address, StorageKeys: append([]string(nil), item.StorageKeys...)}
	}
	return domain.EthereumTransaction{
		Type: value.Type, ChainID: value.ChainID, Nonce: value.Nonce, GasLimit: value.GasLimit,
		GasPrice: value.GasPrice, MaxPriorityFeePerGas: value.MaxPriorityFeePerGas,
		MaxFeePerGas: value.MaxFeePerGas, To: value.To, Value: value.Value, Data: value.Data,
		AccessList: accessList,
	}
}

func signingPayload(format domain.DataFormat, raw json.RawMessage) ([]byte, error) {
	switch format {
	case domain.DataFormatJSON:
		if len(raw) == 0 {
			return nil, errors.New("payload is required")
		}
		var obj map[string]interface{}
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, errors.New("payload must be a valid JSON object")
		}
		return append([]byte(nil), raw...), nil
	case domain.DataFormatRaw:
		var encoded string
		if err := json.Unmarshal(raw, &encoded); err != nil || encoded == "" {
			return nil, errors.New("RAW payload must be a base64url string")
		}
		decoded, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			decoded, err = base64.URLEncoding.DecodeString(encoded)
		}
		if err != nil {
			return nil, errors.New("RAW payload must be valid base64url")
		}
		return decoded, nil
	default:
		return nil, errors.New("format must be JSON or RAW")
	}
}

func verifyHTTPResult(result *portin.VerifyResult) *verifyOutput {
	out := &verifyOutput{}
	out.Body.Valid = result.Valid
	if len(result.Digest) > 0 {
		out.Body.Digest = hex0x(result.Digest)
	}
	return out
}

func parseHex(value string) ([]byte, error) {
	value = strings.TrimPrefix(value, "0x")
	value = strings.TrimPrefix(value, "0X")
	if len(value)%2 != 0 {
		return nil, errors.New("odd hex length")
	}
	return hex.DecodeString(value)
}

func hex0x(value []byte) string { return "0x" + hex.EncodeToString(value) }
