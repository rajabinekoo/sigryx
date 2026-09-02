package http

import (
	"context"
	"encoding/hex"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
)

type createWalletInput struct {
	Body struct {
		KeyRootID  string            `json:"key_root_id" doc:"HD key root used to derive the wallet."`
		UserID     string            `json:"user_id" doc:"Stable external user identifier. The same user ID and key root always return the same wallet."`
		WalletType domain.WalletType `json:"wallet_type" doc:"Wallet profile. Currently supported: ETHEREUM."`
	}
}

type createWalletOutput struct {
	Body struct {
		ID             string            `json:"id"`
		KeyRootID      string            `json:"key_root_id"`
		UserID         string            `json:"user_id"`
		WalletType     domain.WalletType `json:"wallet_type"`
		Adapter        string            `json:"adapter"`
		DerivationPath string            `json:"derivation_path"`
		PublicKey      string            `json:"public_key"`
		Address        string            `json:"address"`
	}
}

func registerWalletRoutes(api huma.API, wallets portin.WalletUseCase) {
	huma.Register(api, huma.Operation{
		OperationID: "create_wallet",
		Method:      http.MethodPost,
		Path:        "/v1/wallets",
		Summary:     "Create or get HD wallet",
		Description: "Returns the existing wallet when the same key root, wallet type, and user ID are submitted again. Private keys are never persisted or returned.",
		Tags:        []string{"hd-wallet"},
	}, func(ctx context.Context, in *createWalletInput) (*createWalletOutput, error) {
		result, err := wallets.Create(ctx, portin.CreateWalletInput{
			KeyRootID:  in.Body.KeyRootID,
			UserID:     in.Body.UserID,
			WalletType: in.Body.WalletType,
		})
		if err != nil {
			return nil, translate(err)
		}

		out := &createWalletOutput{}
		out.Body.ID = result.ID
		out.Body.KeyRootID = result.KeyRootID
		out.Body.UserID = result.UserID
		out.Body.WalletType = result.WalletType
		out.Body.Adapter = result.Adapter
		out.Body.DerivationPath = result.DerivationPath
		out.Body.PublicKey = "0x" + hex.EncodeToString(result.PublicKey)
		out.Body.Address = result.Address
		return out, nil
	})
}
