package http

import (
	"errors"

	"github.com/danielgtaylor/huma/v2"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/internal/core/service"
	pkgerrors "github.com/rajabinekoo/sigryx/pkg/errors"
	"github.com/rajabinekoo/sigryx/pkg/secretstore"
)

func translate(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, service.ErrInvalidUnsealKeyCount),
		errors.Is(err, service.ErrUnsupportedWalletType),
		errors.Is(err, service.ErrInvalidWalletUserID),
		errors.Is(err, service.ErrInvalidKeyRootID),
		errors.Is(err, service.ErrWalletSchemeMismatch),
		errors.Is(err, secretstore.ErrInvalidUnsealSlot),
		errors.Is(err, secretstore.ErrInvalidUnsealKeySize):
		return huma.Error400BadRequest(err.Error())

	case errors.Is(err, service.ErrInvalidCredential):
		return huma.Error401Unauthorized("invalid unseal credential")

	case errors.Is(err, portout.ErrAlreadyInitialized),
		errors.Is(err, portout.ErrKeyRootAlreadyExists),
		errors.Is(err, portout.ErrWalletAlreadyExists),
		errors.Is(err, portout.ErrDerivationIndexExhausted),
		errors.Is(err, service.ErrNotInitialized),
		errors.Is(err, secretstore.ErrVaultSealed),
		errors.Is(err, secretstore.ErrDuplicateUnsealSlot),
		errors.Is(err, secretstore.ErrVaultAlreadyUnsealed),
		errors.Is(err, secretstore.ErrUnsealConfigurationLocked):
		return huma.Error409Conflict(err.Error())

	case errors.Is(err, portout.ErrKeyRootNotFound),
		errors.Is(err, portout.ErrWalletNotFound):
		return huma.Error404NotFound(err.Error())

	case errors.Is(err, service.ErrCorruptedUnsealSlot):
		return huma.Error500InternalServerError("internal error")
	}

	switch pkgerrors.KindOf(err) {
	case pkgerrors.KindInvalidInput:
		return huma.Error400BadRequest(err.Error())
	case pkgerrors.KindUnauthenticated:
		return huma.Error401Unauthorized(err.Error())
	case pkgerrors.KindPermissionDenied:
		return huma.Error403Forbidden(err.Error())
	case pkgerrors.KindNotFound:
		return huma.Error404NotFound(err.Error())
	case pkgerrors.KindAlreadyExists, pkgerrors.KindConflict:
		return huma.Error409Conflict(err.Error())
	case pkgerrors.KindRateLimited:
		return huma.Error429TooManyRequests(err.Error())
	case pkgerrors.KindUnavailable:
		return huma.Error503ServiceUnavailable(err.Error())
	case pkgerrors.KindInternal:
		return huma.Error500InternalServerError(err.Error())
	default:
		return huma.Error500InternalServerError("internal error")
	}
}
