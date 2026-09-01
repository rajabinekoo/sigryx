package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

func Load[T any](dst *T) error {
	if err := env.Parse(dst); err != nil {
		return fmt.Errorf("config: parse env: %w", err)
	}
	return nil
}
