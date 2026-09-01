package http

import (
	"github.com/danielgtaylor/huma/v2"
	pkgerrors "github.com/rajabinekoo/sigryx/pkg/errors"
)

func translate(err error) error {
	if err == nil {
		return nil
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
	case pkgerrors.KindAlreadyExists:
		return huma.Error409Conflict(err.Error())
	case pkgerrors.KindConflict:
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
