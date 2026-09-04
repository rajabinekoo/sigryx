package requestmeta

import (
	"context"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

type Metadata struct {
	RequestID string
	SourceIP  string
	Principal domain.Principal
}

type contextKey struct{}

func With(ctx context.Context, metadata Metadata) context.Context {
	return context.WithValue(ctx, contextKey{}, metadata)
}

func From(ctx context.Context) Metadata {
	metadata, _ := ctx.Value(contextKey{}).(Metadata)
	return metadata
}
