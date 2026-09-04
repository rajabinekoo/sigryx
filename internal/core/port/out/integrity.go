package out

import (
	"context"
	"errors"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

var (
	ErrSigningRecordNotFound = errors.New("signing record not found")
	ErrSigningRecordExists   = errors.New("signing record already exists")
)

type SigningRecordRepository interface {
	Get(context.Context, string, string) (*domain.SigningRecord, error)
	Create(context.Context, domain.SigningRecord) error
}

type AlertSink interface {
	Send(context.Context, domain.Alert) error
}
