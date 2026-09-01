package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/rajabinekoo/sigryx/internal/secretstore"
	"github.com/rajabinekoo/sigryx/internal/securemem"
)

func main() {
	if err := run(); err != nil {
		slog.New(
			slog.NewJSONHandler(
				os.Stderr,
				nil,
			),
		).Error(
			"server exited with error",
			slog.Any(
				"err",
				err,
			),
		)
		os.Exit(1)
	}
}

func run() error {
	if err := securemem.Initialize(); err != nil {
		return fmt.Errorf(
			"initialize secure memory: %w",
			err,
		)
	}

	secretStore, err := secretstore.New(3)
	if err != nil {
		return err
	}
	defer secretStore.Clear()

	return nil
}
