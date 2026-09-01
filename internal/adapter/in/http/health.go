package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

type healthInput struct {
}

type healthOutput struct {
	Body struct {
		Message string `json:"message"`
	}
}

func registerHealthRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "health_check",
		Method:      http.MethodGet,
		Path:        "/v1/health",
		Summary:     "Health check",
		Tags:        []string{"health"},
	}, func(ctx context.Context, in *healthInput) (*healthOutput, error) {
		msg := "it's healthy"
		return &healthOutput{Body: struct {
			Message string `json:"message"`
		}{Message: msg}}, nil
	})
}
