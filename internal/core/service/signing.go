package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/internal/core/requestmeta"
	"github.com/rajabinekoo/sigryx/pkg/canonicaljson"
	"github.com/rajabinekoo/sigryx/pkg/idgen"
	"github.com/rajabinekoo/sigryx/pkg/secretstore"
	"github.com/rajabinekoo/sigryx/pkg/signingrecord"
)

var (
	ErrInvalidWalletID         = errors.New("signing: wallet id is required")
	ErrSigningAdapterMismatch  = errors.New("signing: wallet adapter is not supported")
	ErrInvalidDataFormat       = errors.New("signing: invalid data format")
	ErrSigningContextRequired  = errors.New("signing: context is required")
	ErrEmptySigningPayload     = errors.New("signing: payload is required")
	ErrInvalidJSONPayload      = errors.New("signing: invalid JSON payload")
	ErrIntegrityObjectID       = errors.New("signing: integrity object_id is required")
	ErrIntegrityFields         = errors.New("signing: integrity_fields are required")
	ErrIntegrityUnavailable    = errors.New("signing: integrity signing is not configured")
	ErrIntegritySchemaMismatch = errors.New("signing: integrity field schema does not match original record")
	ErrIntegrityValueMismatch  = errors.New("signing: protected integrity values do not match original record")
	ErrIntegrityWalletMismatch = errors.New("signing: integrity wallet does not match original record")
	ErrSigningRecordTampered   = errors.New("signing: signing record failed integrity validation")
)

type IntegrityDependencies struct {
	Records portout.SigningRecordRepository
	Audit   portout.AuditWriter
	Alerts  portout.AlertSink
}

type SigningService struct {
	wallets   portout.WalletRepository
	keyRoots  portout.KeyRootRepository
	secrets   *secretstore.Store
	adapter   portout.SigningAdapter
	integrity IntegrityDependencies
}

func NewSigningService(
	wallets portout.WalletRepository,
	keyRoots portout.KeyRootRepository,
	secrets *secretstore.Store,
	adapter portout.SigningAdapter,
	integrity IntegrityDependencies,
) *SigningService {
	return &SigningService{
		wallets:   wallets,
		keyRoots:  keyRoots,
		secrets:   secrets,
		adapter:   adapter,
		integrity: integrity,
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

func (s *SigningService) SignIntegrity(
	ctx context.Context,
	input portin.SignIntegrityInput,
) (*portin.SignIntegrityResult, error) {
	canonical, fields, err := integritySelection(input.Context, input.ObjectID, input.Payload, input.IntegrityFields)
	if err != nil {
		return nil, err
	}
	if !s.integrityConfigured() {
		return nil, ErrIntegrityUnavailable
	}

	wallet, err := s.walletForSigning(ctx, input.WalletID)
	if err != nil {
		return nil, err
	}

	existing, err := s.integrity.Records.Get(ctx, input.Context, input.ObjectID)
	if err == nil {
		return s.reuseIntegrityRecord(ctx, input, wallet, canonical, fields, existing)
	}
	if !errors.Is(err, portout.ErrSigningRecordNotFound) {
		return nil, err
	}

	digest := integrityDigest(input.Context, input.ObjectID, canonical)
	var signature []byte
	err = s.withPrivateKey(ctx, wallet, func(privateKey []byte) error {
		var signErr error
		signature, signErr = s.adapter.SignDigest(privateKey, digest)
		return signErr
	})
	if err != nil {
		return nil, err
	}
	if len(signature) != 64 || !s.adapter.VerifyDigest(wallet.PublicKey, digest, signature) {
		return nil, portout.ErrInvalidSignature
	}

	encrypted, err := s.sealSigningRecord(input.Context, input.ObjectID, signingrecord.Record{
		WalletID: wallet.ID, IntegrityFields: fields, CanonicalData: canonical,
		Digest: digest, Signature: signature,
	})
	if err != nil {
		return nil, err
	}
	record := domain.SigningRecord{
		ID: idgen.New(), Context: input.Context, ObjectID: input.ObjectID,
		EncryptedRecord: encrypted, CreatedAt: time.Now().UTC(),
	}
	if err := s.integrity.Records.Create(ctx, record); err != nil {
		if !errors.Is(err, portout.ErrSigningRecordExists) {
			return nil, err
		}
		// A concurrent request won the unique (context, object_id) race.
		// Re-read the committed truth and evaluate against it.
		existing, getErr := s.integrity.Records.Get(ctx, input.Context, input.ObjectID)
		if getErr != nil {
			return nil, getErr
		}
		return s.reuseIntegrityRecord(ctx, input, wallet, canonical, fields, existing)
	}

	return &portin.SignIntegrityResult{Signature: signature, Digest: digest}, nil
}

func (s *SigningService) VerifyIntegrity(
	ctx context.Context,
	input portin.VerifyIntegrityInput,
) (*portin.VerifyIntegrityResult, error) {
	canonical, fields, err := integritySelection(input.Context, input.ObjectID, input.Payload, input.IntegrityFields)
	if err != nil {
		return nil, err
	}
	if !s.integrityConfigured() {
		return nil, ErrIntegrityUnavailable
	}
	if !s.secrets.IsUnsealed() {
		return nil, secretstore.ErrVaultSealed
	}
	if len(input.Signature) != 64 {
		return nil, portout.ErrInvalidSignature
	}

	wallet, err := s.walletForVerification(ctx, input.WalletID)
	if err != nil {
		return nil, err
	}
	digest := integrityDigest(input.Context, input.ObjectID, canonical)
	signatureValid := s.adapter.VerifyDigest(wallet.PublicKey, digest, input.Signature)

	record, err := s.integrity.Records.Get(ctx, input.Context, input.ObjectID)
	if errors.Is(err, portout.ErrSigningRecordNotFound) {
		s.integrityIncident(ctx, "SIGNING_RECORD_NOT_FOUND", input.Context, input.ObjectID, map[string]any{
			"incoming_digest": hex0xString(digest),
		})
		return &portin.VerifyIntegrityResult{
			Valid: false, SignatureValid: signatureValid, RecordMatch: false,
			Digest: digest, Reason: "SIGNING_RECORD_NOT_FOUND",
		}, nil
	}
	if err != nil {
		return nil, err
	}

	stored, err := s.openSigningRecord(record.Context, record.ObjectID, record.EncryptedRecord)
	if err != nil {
		s.integrityIncident(ctx, "SIGNING_RECORD_TAMPERED", input.Context, input.ObjectID, nil)
		return &portin.VerifyIntegrityResult{
			Valid: false, SignatureValid: signatureValid, RecordMatch: false,
			Digest: digest, Reason: "SIGNING_RECORD_TAMPERED",
		}, nil
	}

	reason := ""
	recordMatch := true
	switch {
	case stored.WalletID != input.WalletID:
		reason, recordMatch = "WALLET_MISMATCH", false
	case !slices.Equal(stored.IntegrityFields, fields):
		reason, recordMatch = "INTEGRITY_SCHEMA_MISMATCH", false
	case !bytesEqual(stored.CanonicalData, canonical):
		reason, recordMatch = "INTEGRITY_VALUE_MISMATCH", false
	case !bytesEqual(stored.Digest, digest):
		reason, recordMatch = "DIGEST_MISMATCH", false
	case !bytesEqual(stored.Signature, input.Signature):
		reason, recordMatch = "SIGNATURE_MISMATCH", false
	}

	if recordMatch {
		storedWallet, walletErr := s.walletForVerification(ctx, stored.WalletID)
		if walletErr != nil || !s.adapter.VerifyDigest(storedWallet.PublicKey, stored.Digest, stored.Signature) {
			reason, recordMatch = "STORED_SIGNATURE_INVALID", false
		}
	}
	if !recordMatch {
		details := map[string]any{
			"stored_digest":   hex0xString(stored.Digest),
			"incoming_digest": hex0xString(digest),
		}
		s.integrityIncident(ctx, reason, input.Context, input.ObjectID, details)
	}

	return &portin.VerifyIntegrityResult{
		Valid:          signatureValid && recordMatch,
		SignatureValid: signatureValid,
		RecordMatch:    recordMatch,
		Digest:         digest,
		Reason:         reason,
	}, nil
}

func (s *SigningService) reuseIntegrityRecord(
	ctx context.Context,
	input portin.SignIntegrityInput,
	wallet *domain.Wallet,
	canonical []byte,
	fields []string,
	record *domain.SigningRecord,
) (*portin.SignIntegrityResult, error) {
	stored, err := s.openSigningRecord(record.Context, record.ObjectID, record.EncryptedRecord)
	if err != nil {
		s.integrityIncident(ctx, "SIGNING_RECORD_TAMPERED", input.Context, input.ObjectID, nil)
		return nil, ErrSigningRecordTampered
	}

	switch {
	case stored.WalletID != wallet.ID:
		s.integrityIncident(ctx, "INTEGRITY_WALLET_MISMATCH", input.Context, input.ObjectID, map[string]any{
			"stored_wallet_id":   stored.WalletID,
			"incoming_wallet_id": wallet.ID,
		})
		return nil, ErrIntegrityWalletMismatch
	case !slices.Equal(stored.IntegrityFields, fields):
		s.integrityIncident(ctx, "INTEGRITY_SCHEMA_MISMATCH", input.Context, input.ObjectID, map[string]any{
			"stored_fields":   stored.IntegrityFields,
			"incoming_fields": fields,
		})
		return nil, ErrIntegritySchemaMismatch
	case !bytesEqual(stored.CanonicalData, canonical):
		s.integrityIncident(ctx, "INTEGRITY_VALUE_MISMATCH", input.Context, input.ObjectID, map[string]any{
			"stored_digest":   hex0xString(stored.Digest),
			"incoming_digest": hex0xString(integrityDigest(input.Context, input.ObjectID, canonical)),
		})
		return nil, ErrIntegrityValueMismatch
	}

	expectedDigest := integrityDigest(input.Context, input.ObjectID, stored.CanonicalData)
	if !bytesEqual(expectedDigest, stored.Digest) || !s.adapter.VerifyDigest(wallet.PublicKey, stored.Digest, stored.Signature) {
		s.integrityIncident(ctx, "SIGNING_RECORD_TAMPERED", input.Context, input.ObjectID, nil)
		return nil, ErrSigningRecordTampered
	}

	return &portin.SignIntegrityResult{
		Signature: append([]byte(nil), stored.Signature...),
		Digest:    append([]byte(nil), stored.Digest...),
		Reused:    true,
	}, nil
}

func (s *SigningService) integrityConfigured() bool {
	return s.integrity.Records != nil
}

func (s *SigningService) sealSigningRecord(
	contextName string,
	objectID string,
	record signingrecord.Record,
) ([]byte, error) {
	var encrypted []byte
	err := s.secrets.WithVaultEncryptionKey(func(vaultKey []byte) error {
		var sealErr error
		encrypted, sealErr = signingrecord.Seal(vaultKey, contextName, objectID, record)
		return sealErr
	})
	if err != nil {
		return nil, err
	}
	return encrypted, nil
}

func (s *SigningService) openSigningRecord(
	contextName string,
	objectID string,
	encrypted []byte,
) (*signingrecord.Record, error) {
	var record *signingrecord.Record
	err := s.secrets.WithVaultEncryptionKey(func(vaultKey []byte) error {
		var openErr error
		record, openErr = signingrecord.Open(vaultKey, contextName, objectID, encrypted)
		return openErr
	})
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (s *SigningService) integrityIncident(ctx context.Context, code, contextName, objectID string, details map[string]any) {
	metadata := requestmeta.From(ctx)
	if details == nil {
		details = map[string]any{}
	}
	details["context"] = contextName
	details["object_id"] = objectID

	incidentCtx := context.WithoutCancel(ctx)
	if s.integrity.Audit != nil {
		_ = s.integrity.Audit.Append(incidentCtx, domain.AuditEvent{
			ID: idgen.New(), OccurredAt: time.Now().UTC(), ActorType: string(metadata.Principal.Kind),
			ActorID: metadata.Principal.ID, SessionID: metadata.Principal.SessionID,
			Action: "security.integrity_violation", Outcome: domain.AuditOutcomeBlocked,
			SourceIP: metadata.SourceIP, RequestID: metadata.RequestID,
			Details: mergeDetails(details, map[string]any{"code": code}),
		})
	}
	if s.integrity.Alerts != nil {
		if err := s.integrity.Alerts.Send(incidentCtx, domain.Alert{
			Severity: "CRITICAL", Code: code, OccurredAt: time.Now().UTC(),
			ActorID: metadata.Principal.ID, SourceIP: metadata.SourceIP, RequestID: metadata.RequestID,
			Context: contextName, ObjectID: objectID, Details: details,
		}); err != nil && s.integrity.Audit != nil {
			_ = s.integrity.Audit.Append(incidentCtx, domain.AuditEvent{
				ID: idgen.New(), OccurredAt: time.Now().UTC(), ActorType: string(metadata.Principal.Kind),
				ActorID: metadata.Principal.ID, SessionID: metadata.Principal.SessionID,
				Action: "security.alert_delivery_failed", Outcome: domain.AuditOutcomeFailed,
				SourceIP: metadata.SourceIP, RequestID: metadata.RequestID,
				Details: map[string]any{"code": code, "context": contextName, "object_id": objectID},
			})
		}
	}
}

func integritySelection(contextName, objectID string, payload []byte, fields []string) ([]byte, []string, error) {
	if contextName == "" {
		return nil, nil, ErrSigningContextRequired
	}
	if objectID == "" {
		return nil, nil, ErrIntegrityObjectID
	}
	if len(fields) == 0 {
		return nil, nil, ErrIntegrityFields
	}
	if len(payload) == 0 {
		return nil, nil, ErrEmptySigningPayload
	}
	canonical, normalized, err := canonicaljson.Select(payload, fields)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidJSONPayload, err)
	}
	return canonical, normalized, nil
}

func integrityDigest(contextName, objectID string, canonical []byte) []byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte("sigryx:integrity-sign:v1"))
	writeFramed(hash, []byte(contextName))
	writeFramed(hash, []byte(objectID))
	writeFramed(hash, canonical)
	return hash.Sum(nil)
}

func hex0xString(value []byte) string {
	return "0x" + hex.EncodeToString(value)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

func mergeDetails(base, extra map[string]any) map[string]any {
	result := make(map[string]any, len(base)+len(extra))
	for key, value := range base {
		result[key] = value
	}
	for key, value := range extra {
		result[key] = value
	}
	return result
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
