// Package hdseed owns generation rules for blockchain-agnostic HD master seeds.
package hdseed

import (
	"fmt"

	"github.com/rajabinekoo/sigryx/pkg/securemem"
)

// Size is the size of every Sigryx HD master seed in bytes.
//
// 32 bytes provides 256 bits of CSPRNG entropy and is valid input for BIP32.
// The seed itself is intentionally blockchain-agnostic.
const Size = 32

// Generate creates a new HD master seed directly inside protected memory.
// No plaintext seed copy is created on the Go heap by this function.
func Generate() (*securemem.Secret, error) {
	seed, err := securemem.Random(Size)
	if err != nil {
		return nil, fmt.Errorf("generate hd master seed: %w", err)
	}

	return seed, nil
}
