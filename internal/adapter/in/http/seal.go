package http

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/rajabinekoo/sigryx/internal/core/domain"
	portin "github.com/rajabinekoo/sigryx/internal/core/port/in"
)

type initializeVaultInput struct {
	Body struct {
		UnsealKeyCount int `json:"unseal_key_count" minimum:"1" doc:"Number of N-of-N unseal credentials to create."`
	}
}

type unsealCredentialResponse struct {
	SlotID        int    `json:"slot_id"`
	UnsealPayload string `json:"unseal_payload" doc:"Base64URL encoded wrapped unseal key."`
	OwnerKey      string `json:"owner_key" doc:"Base64URL encoded owner-held secret. Store it securely; Sigryx does not persist it."`
}

type initializeVaultOutput struct {
	Body struct {
		State       domain.SealState           `json:"state"`
		Credentials []unsealCredentialResponse `json:"credentials"`
	}
}

type submitUnsealInput struct {
	Body struct {
		SlotID        int    `json:"slot_id" minimum:"1"`
		UnsealPayload string `json:"unseal_payload"`
		OwnerKey      string `json:"owner_key"`
	}
}

type sealStatusOutput struct {
	Body struct {
		State     domain.SealState `json:"state"`
		Submitted int              `json:"submitted"`
		Required  int              `json:"required"`
	}
}

type emptyInput struct{}

type sealOutput struct {
	Body struct {
		State domain.SealState `json:"state"`
	}
}

func registerSealRoutes(api huma.API, seal portin.SealUseCase) {
	huma.Register(api, huma.Operation{
		OperationID: "initialize_vault",
		Method:      http.MethodPost,
		Path:        "/v1/vault/init",
		Summary:     "Initialize vault",
		Description: "Creates the N-of-N unseal credentials. The returned owner keys are shown only once and are never persisted by Sigryx.",
		Tags:        []string{"vault"},
	}, func(ctx context.Context, in *initializeVaultInput) (*initializeVaultOutput, error) {
		result, err := seal.Initialize(ctx, portin.InitializeVaultInput{
			UnsealKeyCount: in.Body.UnsealKeyCount,
		})
		if err != nil {
			return nil, translate(err)
		}
		defer wipeCredentials(result.Credentials)

		out := &initializeVaultOutput{}
		out.Body.State = domain.SealStateSealed
		out.Body.Credentials = make([]unsealCredentialResponse, 0, len(result.Credentials))

		for _, credential := range result.Credentials {
			out.Body.Credentials = append(out.Body.Credentials, unsealCredentialResponse{
				SlotID:        int(credential.Payload.SlotID),
				UnsealPayload: base64.RawURLEncoding.EncodeToString(credential.Payload.WrappedKey),
				OwnerKey:      base64.RawURLEncoding.EncodeToString(credential.OwnerSecret),
			})
		}

		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "unseal_vault",
		Method:      http.MethodPost,
		Path:        "/v1/vault/unseal",
		Summary:     "Submit unseal credential",
		Description: "Submits one N-of-N unseal credential. The vault becomes unsealed after every configured slot has been submitted successfully.",
		Tags:        []string{"vault"},
	}, func(ctx context.Context, in *submitUnsealInput) (*sealStatusOutput, error) {
		wrappedKey, err := decodeBase64URL("unseal_payload", in.Body.UnsealPayload)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}

		ownerKey, err := decodeBase64URL("owner_key", in.Body.OwnerKey)
		if err != nil {
			return nil, huma.Error400BadRequest(err.Error())
		}
		// SubmitUnsealCredential owns ownerKey and wipes it before returning.

		status, err := seal.SubmitUnsealCredential(ctx, portin.SubmitUnsealCredentialInput{
			SlotID:      domain.UnsealSlotID(in.Body.SlotID),
			WrappedKey:  wrappedKey,
			OwnerSecret: ownerKey,
		})
		if err != nil {
			return nil, translate(err)
		}

		return newSealStatusOutput(status), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get_vault_status",
		Method:      http.MethodGet,
		Path:        "/v1/vault/status",
		Summary:     "Get vault status",
		Tags:        []string{"vault"},
	}, func(ctx context.Context, _ *emptyInput) (*sealStatusOutput, error) {
		status, err := seal.Status(ctx)
		if err != nil {
			return nil, translate(err)
		}

		return newSealStatusOutput(status), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "seal_vault",
		Method:      http.MethodPost,
		Path:        "/v1/vault/seal",
		Summary:     "Seal vault",
		Description: "Destroys all runtime secret material, including partial unseal state and the vault encryption key.",
		Tags:        []string{"vault"},
	}, func(ctx context.Context, _ *emptyInput) (*sealOutput, error) {
		if err := seal.Seal(ctx); err != nil {
			return nil, translate(err)
		}

		out := &sealOutput{}
		out.Body.State = domain.SealStateSealed
		return out, nil
	})
}

func newSealStatusOutput(status portin.SealStatus) *sealStatusOutput {
	out := &sealStatusOutput{}
	out.Body.State = status.State
	out.Body.Submitted = status.Submitted
	out.Body.Required = status.Required
	return out
}

func decodeBase64URL(name, value string) ([]byte, error) {
	if value == "" {
		return nil, fmt.Errorf("%s is required", name)
	}

	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be valid base64url without padding", name)
	}

	return decoded, nil
}

func wipeCredentials(credentials []domain.UnsealCredential) {
	for i := range credentials {
		clear(credentials[i].OwnerSecret)
	}
}
