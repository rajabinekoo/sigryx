package http

import (
	"errors"

	"github.com/danielgtaylor/huma/v2"
	portout "github.com/rajabinekoo/sigryx/internal/core/port/out"
	"github.com/rajabinekoo/sigryx/internal/core/service"
	pkgerrors "github.com/rajabinekoo/sigryx/pkg/errors"
	"github.com/rajabinekoo/sigryx/pkg/recovery"
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
		errors.Is(err, service.ErrInvalidWalletID),
		errors.Is(err, service.ErrSigningAdapterMismatch),
		errors.Is(err, service.ErrInvalidDataFormat),
		errors.Is(err, service.ErrSigningContextRequired),
		errors.Is(err, service.ErrEmptySigningPayload),
		errors.Is(err, service.ErrInvalidJSONPayload),
		errors.Is(err, portout.ErrInvalidTransaction),
		errors.Is(err, portout.ErrInvalidTypedData),
		errors.Is(err, portout.ErrInvalidSignature),
		errors.Is(err, secretstore.ErrInvalidUnsealSlot),
		errors.Is(err, secretstore.ErrInvalidUnsealKeySize),
		errors.Is(err, service.ErrInvalidUsername),
		errors.Is(err, service.ErrInvalidRoleName),
		errors.Is(err, service.ErrInvalidPermission),
		errors.Is(err, service.ErrInvalidCIDR),
		errors.Is(err, service.ErrInvalidServiceName),
		errors.Is(err, service.ErrInvalidAccessID),
		errors.Is(err, service.ErrInvalidNewPassword),
		errors.Is(err, service.ErrRecoveryInvalidKeyRoot),
		errors.Is(err, recovery.ErrInvalidKey),
		errors.Is(err, recovery.ErrInvalidBackup),
		errors.Is(err, recovery.ErrUnsupportedVersion):
		return huma.Error400BadRequest(err.Error())

	case errors.Is(err, service.ErrInvalidCredential):
		return huma.Error401Unauthorized("invalid unseal credential")

	case errors.Is(err, service.ErrInvalidSetupToken),
		errors.Is(err, service.ErrInvalidCredentials),
		errors.Is(err, service.ErrInvalidRefreshToken),
		errors.Is(err, service.ErrSessionRevoked),
		errors.Is(err, service.ErrSessionExpired),
		errors.Is(err, service.ErrCurrentPassword):
		return huma.Error401Unauthorized(err.Error())

	case errors.Is(err, service.ErrPermissionDenied),
		errors.Is(err, service.ErrIPNotAllowed),
		errors.Is(err, service.ErrInactivePrincipal),
		errors.Is(err, service.ErrRootAdminImmutable),
		errors.Is(err, service.ErrUserPrincipalOnly),
		errors.Is(err, service.ErrRootNetworkOnly),
		errors.Is(err, service.ErrRecoveryRootAdminRequired):
		return huma.Error403Forbidden(err.Error())

	case errors.Is(err, portout.ErrAlreadyInitialized),
		errors.Is(err, portout.ErrKeyRootAlreadyExists),
		errors.Is(err, portout.ErrWalletAlreadyExists),
		errors.Is(err, portout.ErrDerivationIndexExhausted),
		errors.Is(err, service.ErrNotInitialized),
		errors.Is(err, secretstore.ErrVaultSealed),
		errors.Is(err, secretstore.ErrDuplicateUnsealSlot),
		errors.Is(err, secretstore.ErrVaultAlreadyUnsealed),
		errors.Is(err, secretstore.ErrUnsealConfigurationLocked),
		errors.Is(err, service.ErrAlreadySetup),
		errors.Is(err, portout.ErrAccessAlreadyExists),
		errors.Is(err, portout.ErrRecoveryKeyRootConflict),
		errors.Is(err, service.ErrRecoveryNoKeyRoots):
		return huma.Error409Conflict(err.Error())

	case errors.Is(err, portout.ErrKeyRootNotFound),
		errors.Is(err, portout.ErrWalletNotFound),
		errors.Is(err, portout.ErrUserNotFound),
		errors.Is(err, portout.ErrRoleNotFound),
		errors.Is(err, portout.ErrServiceAccountNotFound),
		errors.Is(err, portout.ErrSessionNotFound):
		return huma.Error404NotFound(err.Error())

	case errors.Is(err, service.ErrSetupDisabled):
		return huma.Error503ServiceUnavailable(err.Error())

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
