package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/pkg/idgen"
	"github.com/rajabinekoo/sigryx/pkg/secretstore"
)

var (
	ErrInvalidWalletUserID  = errors.New("wallet: user id is required")
	ErrInvalidKeyRootID     = errors.New("wallet: key root id is required")
	ErrWalletSchemeMismatch = errors.New("wallet: key root derivation scheme is not supported by wallet type")
)

type WalletService struct {
	wallets  portout.WalletRepository
	keyRoots portout.KeyRootRepository
	secrets  *secretstore.Store
	adapter  portout.WalletAdapter
}

func NewWalletService(
	wallets portout.WalletRepository,
	keyRoots portout.KeyRootRepository,
	secrets *secretstore.Store,
	adapter portout.WalletAdapter,
) *WalletService {
	return &WalletService{
		wallets:  wallets,
		keyRoots: keyRoots,
		secrets:  secrets,
		adapter:  adapter,
	}
}

func (s *WalletService) Create(
	ctx context.Context,
	input portin.CreateWalletInput,
) (*portin.WalletResult, error) {
	if input.UserID == "" {
		return nil, ErrInvalidWalletUserID
	}
	if input.KeyRootID == "" {
		return nil, ErrInvalidKeyRootID
	}
	if input.WalletType != s.adapter.WalletType() {
		return nil, ErrUnsupportedWalletType
	}

	adapterName := s.adapter.Name()

	wallet, err := s.wallets.GetByUser(
		ctx,
		input.KeyRootID,
		adapterName,
		input.UserID,
	)
	if err == nil {
		return walletResult(wallet, input.WalletType), nil
	}
	if !errors.Is(err, portout.ErrWalletNotFound) {
		return nil, err
	}

	if !s.secrets.IsUnsealed() {
		return nil, secretstore.ErrVaultSealed
	}

	root, err := s.keyRoots.GetByID(ctx, input.KeyRootID)
	if err != nil {
		return nil, err
	}
	if root.DerivationScheme != s.adapter.DerivationScheme() {
		return nil, ErrWalletSchemeMismatch
	}

	index, err := s.wallets.NextIndex(ctx, root.ID, adapterName)
	if err != nil {
		return nil, err
	}

	var derived portout.DerivedWallet
	err = withKeyRootSeed(s.secrets, root, func(seed []byte) error {
		var deriveErr error
		derived, deriveErr = s.adapter.Derive(seed, index)
		return deriveErr
	})
	if err != nil {
		return nil, err
	}

	wallet = &domain.Wallet{
		ID:             idgen.New(),
		KeyRootID:      root.ID,
		UserID:         input.UserID,
		Adapter:        adapterName,
		DerivationPath: derived.DerivationPath,
		PublicKey:      derived.PublicKey,
		Address:        derived.Address,
	}

	if err := s.wallets.Create(ctx, *wallet); err != nil {
		if errors.Is(err, portout.ErrWalletAlreadyExists) {
			existing, getErr := s.wallets.GetByUser(
				ctx,
				root.ID,
				adapterName,
				input.UserID,
			)
			if getErr == nil {
				return walletResult(existing, input.WalletType), nil
			}
			if errors.Is(getErr, portout.ErrWalletNotFound) {
				return nil, err
			}
			return nil, fmt.Errorf("load concurrently created wallet: %w", getErr)
		}
		return nil, err
	}

	return walletResult(wallet, input.WalletType), nil
}

func walletResult(
	wallet *domain.Wallet,
	walletType domain.WalletType,
) *portin.WalletResult {
	return &portin.WalletResult{
		ID:             wallet.ID,
		KeyRootID:      wallet.KeyRootID,
		UserID:         wallet.UserID,
		WalletType:     walletType,
		Adapter:        wallet.Adapter,
		DerivationPath: wallet.DerivationPath,
		PublicKey:      append([]byte(nil), wallet.PublicKey...),
		Address:        wallet.Address,
	}
}

var _ portin.WalletUseCase = (*WalletService)(nil)
