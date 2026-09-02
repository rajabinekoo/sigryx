package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
)

type createKeyRootInput struct {
	Body struct {
		WalletType domain.WalletType `json:"wallet_type" doc:"Wallet profile used to select the HD derivation scheme. Currently supported: ETHEREUM."`
	}
}

type createKeyRootOutput struct {
	Body struct {
		ID               string                  `json:"id"`
		WalletType       domain.WalletType       `json:"wallet_type"`
		DerivationScheme domain.DerivationScheme `json:"derivation_scheme"`
	}
}

type getListOfKeyRootsOutput struct {
	Body struct {
		KeyRoots []portin.KeyRootResult `json:"key_roots"`
	}
}

func registerKeyRootRoutes(api huma.API, keyRoots portin.KeyRootUseCase) {
	huma.Register(api, huma.Operation{
		OperationID: "create_key_root",
		Method:      http.MethodPost,
		Path:        "/v1/key-roots",
		Summary:     "Create HD key root",
		Description: "Generates a blockchain-agnostic HD master seed in protected memory, seals it with the unsealed Vault encryption key, and persists only the encrypted key root. The plaintext master seed is never returned by the API.",
		Tags:        []string{"hd-wallet"},
	}, func(ctx context.Context, in *createKeyRootInput) (*createKeyRootOutput, error) {
		result, err := keyRoots.Create(ctx, portin.CreateKeyRootInput{
			WalletType: in.Body.WalletType,
		})
		if err != nil {
			return nil, translate(err)
		}

		out := &createKeyRootOutput{}
		out.Body.ID = result.ID
		out.Body.WalletType = result.WalletType
		out.Body.DerivationScheme = result.DerivationScheme
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get_key_roots",
		Method:      http.MethodGet,
		Path:        "/v1/key-roots",
		Summary:     "List HD key roots",
		Description: "Lists persisted HD key roots. This endpoint returns public metadata and works while the vault is sealed.",
		Tags:        []string{"hd-wallet"},
	}, func(ctx context.Context, in *emptyInput) (*getListOfKeyRootsOutput, error) {
		result, err := keyRoots.GetAll(ctx)
		if err != nil {
			return nil, translate(err)
		}

		out := &getListOfKeyRootsOutput{}
		out.Body.KeyRoots = result
		return out, nil
	})
}
