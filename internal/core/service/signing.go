package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/pkg/canonicaljson"
	"github.com/rajabinekoo/sigryx/pkg/secretstore"
)

var (
	ErrInvalidWalletID        = errors.New("signing: wallet id is required")
	ErrSigningAdapterMismatch = errors.New("signing: wallet adapter is not supported")
	ErrInvalidDataFormat      = errors.New("signing: invalid data format")
	ErrSigningContextRequired = errors.New("signing: context is required")
	ErrEmptySigningPayload    = errors.New("signing: payload is required")
	ErrInvalidJSONPayload     = errors.New("signing: invalid JSON payload")
)

type SigningService struct {
	wallets  portout.WalletRepository
	keyRoots portout.KeyRootRepository
	secrets  *secretstore.Store
	adapter  portout.SigningAdapter
}

func NewSigningService(
	wallets portout.WalletRepository,
	keyRoots portout.KeyRootRepository,
	secrets *secretstore.Store,
	adapter portout.SigningAdapter,
) *SigningService {
	return &SigningService{
		wallets:  wallets,
		keyRoots: keyRoots,
		secrets:  secrets,
		adapter:  adapter,
	}
}

func (s *SigningService) SignTransaction(
	ctx context.Context,
	input portin.SignTransactionInput,
) (*portin.SignTransactionResult, error) {
	wallet, err := s.walletForSigning(ctx, input.WalletID)
	if err != nil {
		return nil, err
	}

	var signed portout.TransactionSignature
	err = s.withPrivateKey(ctx, wallet, func(privateKey []byte) error {
		var signErr error
		signed, signErr = s.adapter.SignTransaction(privateKey, input.Transaction)
		return signErr
	})
	if err != nil {
		return nil, err
	}

	return &portin.SignTransactionResult{
		RawTransaction:  signed.RawTransaction,
		TransactionHash: signed.Hash,
		R:               signed.R,
		S:               signed.S,
		YParity:         signed.YParity,
	}, nil
}

func (s *SigningService) VerifyTransaction(
	ctx context.Context,
	input portin.VerifyTransactionInput,
) (*portin.VerifyResult, error) {
	wallet, err := s.walletForVerification(ctx, input.WalletID)
	if err != nil {
		return nil, err
	}
	if len(input.RawTransaction) == 0 {
		return nil, ErrEmptySigningPayload
	}

	valid, err := s.adapter.VerifyTransaction(wallet.PublicKey, input.RawTransaction)
	if err != nil {
		return nil, err
	}
	return &portin.VerifyResult{Valid: valid}, nil
}

func (s *SigningService) SignTypedData(
	ctx context.Context,
	input portin.SignTypedDataInput,
) (*portin.SignTypedDataResult, error) {
	if len(input.TypedData) == 0 {
		return nil, ErrEmptySigningPayload
	}
	wallet, err := s.walletForSigning(ctx, input.WalletID)
	if err != nil {
		return nil, err
	}

	var signature, digest []byte
	err = s.withPrivateKey(ctx, wallet, func(privateKey []byte) error {
		var signErr error
		signature, digest, signErr = s.adapter.SignTypedData(privateKey, input.TypedData)
		return signErr
	})
	if err != nil {
		return nil, err
	}
	return &portin.SignTypedDataResult{Signature: signature, Digest: digest}, nil
}

func (s *SigningService) VerifyTypedData(
	ctx context.Context,
	input portin.VerifyTypedDataInput,
) (*portin.VerifyResult, error) {
	if len(input.TypedData) == 0 || len(input.Signature) == 0 {
		return nil, ErrEmptySigningPayload
	}
	wallet, err := s.walletForVerification(ctx, input.WalletID)
	if err != nil {
		return nil, err
	}

	valid, digest, err := s.adapter.VerifyTypedData(wallet.PublicKey, input.TypedData, input.Signature)
	if err != nil {
		return nil, err
	}
	return &portin.VerifyResult{Valid: valid, Digest: digest}, nil
}

func (s *SigningService) SignData(
	ctx context.Context,
	input portin.SignDataInput,
) (*portin.SignDataResult, error) {
	digest, err := genericDigest(input.Context, input.Format, input.Payload)
	if err != nil {
		return nil, err
	}
	wallet, err := s.walletForSigning(ctx, input.WalletID)
	if err != nil {
		return nil, err
	}

	var signature []byte
	err = s.withPrivateKey(ctx, wallet, func(privateKey []byte) error {
		var signErr error
		signature, signErr = s.adapter.SignDigest(privateKey, digest)
		return signErr
	})
	if err != nil {
		return nil, err
	}
	return &portin.SignDataResult{Signature: signature, Digest: digest}, nil
}

func (s *SigningService) VerifyData(
	ctx context.Context,
	input portin.VerifyDataInput,
) (*portin.VerifyResult, error) {
	digest, err := genericDigest(input.Context, input.Format, input.Payload)
	if err != nil {
		return nil, err
	}
	if len(input.Signature) != 64 {
		return nil, portout.ErrInvalidSignature
	}
	wallet, err := s.walletForVerification(ctx, input.WalletID)
	if err != nil {
		return nil, err
	}
	return &portin.VerifyResult{
		Valid:  s.adapter.VerifyDigest(wallet.PublicKey, digest, input.Signature),
		Digest: digest,
	}, nil
}

func (s *SigningService) walletForSigning(ctx context.Context, walletID string) (*domain.Wallet, error) {
	if !s.secrets.IsUnsealed() {
		return nil, secretstore.ErrVaultSealed
	}
	return s.walletForVerification(ctx, walletID)
}

func (s *SigningService) walletForVerification(ctx context.Context, walletID string) (*domain.Wallet, error) {
	if walletID == "" {
		return nil, ErrInvalidWalletID
	}
	wallet, err := s.wallets.GetByID(ctx, walletID)
	if err != nil {
		return nil, err
	}
	if wallet.Adapter != s.adapter.Name() {
		return nil, ErrSigningAdapterMismatch
	}
	return wallet, nil
}

func (s *SigningService) withPrivateKey(
	ctx context.Context,
	wallet *domain.Wallet,
	fn func([]byte) error,
) error {
	root, err := s.keyRoots.GetByID(ctx, wallet.KeyRootID)
	if err != nil {
		return err
	}
	if root.DerivationScheme != s.adapter.DerivationScheme() {
		return ErrWalletSchemeMismatch
	}

	return withKeyRootSeed(s.secrets, root, func(seed []byte) error {
		return s.adapter.WithPrivateKey(seed, wallet.DerivationPath, fn)
	})
}

func genericDigest(contextName string, format domain.DataFormat, payload []byte) ([]byte, error) {
	if contextName == "" {
		return nil, ErrSigningContextRequired
	}
	if len(payload) == 0 {
		return nil, ErrEmptySigningPayload
	}

	data := payload
	switch format {
	case domain.DataFormatRaw:
	case domain.DataFormatJSON:
		canonical, err := canonicaljson.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidJSONPayload, err)
		}
		data = canonical
	default:
		return nil, ErrInvalidDataFormat
	}

	hash := sha256.New()
	_, _ = hash.Write([]byte("sigryx:generic-sign:v1"))
	writeFramed(hash, []byte(contextName))
	writeFramed(hash, []byte(format))
	writeFramed(hash, data)
	return hash.Sum(nil), nil
}

type byteWriter interface {
	Write([]byte) (int, error)
}

func writeFramed(writer byteWriter, value []byte) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = writer.Write(size[:])
	_, _ = writer.Write(value)
}

var _ portin.SigningUseCase = (*SigningService)(nil)
