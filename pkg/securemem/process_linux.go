//go:build linux

package securemem

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// Initialize prepares libsodium and applies Linux process hardening.
//
// Call this once at the very beginning of the application,
// before loading or generating any secret material.
func Initialize() error {
	if err := initializeSodium(); err != nil {
		return fmt.Errorf(
			"initialize libsodium: %w",
			err,
		)
	}

	if err := disableCoreDumps(); err != nil {
		return fmt.Errorf(
			"disable core dumps: %w",
			err,
		)
	}

	if err := disableDumpable(); err != nil {
		return fmt.Errorf(
			"disable process dumpability: %w",
			err,
		)
	}

	return nil
}

func disableCoreDumps() error {
	return unix.Setrlimit(
		unix.RLIMIT_CORE,
		&unix.Rlimit{
			Cur: 0,
			Max: 0,
		},
	)
}

func disableDumpable() error {
	return unix.Prctl(
		unix.PR_SET_DUMPABLE,
		0,
		0,
		0,
		0,
	)
}
