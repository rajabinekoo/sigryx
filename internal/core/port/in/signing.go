package in

import (
	"context"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

type SignTransactionInput struct {
	WalletID    string
	Transaction domain.EthereumTransaction
}

type SignTransactionResult struct {
	RawTransaction  []byte
	TransactionHash []byte
	R               []byte
	S               []byte
	YParity         uint8
}

type VerifyTransactionInput struct {
	WalletID       string
	RawTransaction []byte
}

type SignTypedDataInput struct {
	WalletID  string
	TypedData []byte
}

type SignTypedDataResult struct {
	Signature []byte
	Digest    []byte
}

type VerifyTypedDataInput struct {
	WalletID  string
	TypedData []byte
	Signature []byte
}

type VerifyResult struct {
	Valid  bool
	Digest []byte
}

type SignDataInput struct {
	WalletID string
	Context  string
	Format   domain.DataFormat
	Payload  []byte
}

type SignDataResult struct {
	Signature []byte
	Digest    []byte
}

type VerifyDataInput struct {
	WalletID  string
	Context   string
	Format    domain.DataFormat
	Payload   []byte
	Signature []byte
}

type SignIntegrityInput struct {
	WalletID        string
	Context         string
	ObjectID        string
	Payload         []byte
	IntegrityFields []string
}

type SignIntegrityResult struct {
	Signature []byte
	Digest    []byte
	Reused    bool
}

type VerifyIntegrityInput struct {
	WalletID        string
	Context         string
	ObjectID        string
	Payload         []byte
	IntegrityFields []string
	Signature       []byte
}

type VerifyIntegrityResult struct {
	Valid          bool
	SignatureValid bool
	RecordMatch    bool
	Digest         []byte
	Reason         string
}

type SigningUseCase interface {
	SignTransaction(context.Context, SignTransactionInput) (*SignTransactionResult, error)
	VerifyTransaction(context.Context, VerifyTransactionInput) (*VerifyResult, error)
	SignTypedData(context.Context, SignTypedDataInput) (*SignTypedDataResult, error)
	VerifyTypedData(context.Context, VerifyTypedDataInput) (*VerifyResult, error)
	SignData(context.Context, SignDataInput) (*SignDataResult, error)
	VerifyData(context.Context, VerifyDataInput) (*VerifyResult, error)
	SignIntegrity(context.Context, SignIntegrityInput) (*SignIntegrityResult, error)
	VerifyIntegrity(context.Context, VerifyIntegrityInput) (*VerifyIntegrityResult, error)
}
