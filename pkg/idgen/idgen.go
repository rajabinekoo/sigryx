// Package idgen is the single source for user-facing identifiers across the
// platform. UUIDv7 is the chosen format:
//
//   - 128-bit, native UUID — works with Postgres `uuid` column type
//   - Time-ordered prefix — index locality on inserts, range scans by creation
//   - Public-safe — no node ID leak, no sequential disclosure of growth rates
//
// All services MUST use idgen.New() for new aggregate IDs. Don't roll your own
// UUID/ULID/snowflake in domain code.
package idgen

import "github.com/google/uuid"

// New returns a fresh UUIDv7 in canonical 8-4-4-4-12 hyphenated form.
func New() string {
	return uuid.Must(uuid.NewV7()).String()
}

// Parse returns the parsed UUID for the given string, or an error if invalid.
// Callers that need to validate IDs at adapter boundaries (gRPC request
// validation, HTTP path params) should call this.
func Parse(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
