//go:build linux && cgo

package securemem

/*
#cgo pkg-config: libsodium

#include <sodium.h>
#include <string.h>
*/
import "C"

import (
	"errors"
	"runtime"
	"sync"
	"unsafe"
)

var (
	sodiumOnce    sync.Once
	sodiumInitErr error
)

type region struct {
	ptr  unsafe.Pointer
	size int
}

func initializeSodium() error {
	sodiumOnce.Do(func() {
		if C.sodium_init() < 0 {
			sodiumInitErr = ErrSodiumInitialization
		}
	})

	return sodiumInitErr
}

func newRegionFromBytes(data []byte) (*region, error) {
	if len(data) == 0 {
		return nil, ErrEmptySecret
	}

	if err := initializeSodium(); err != nil {
		return nil, err
	}

	ptr := C.sodium_malloc(C.size_t(len(data)))
	if ptr == nil {
		return nil, ErrAllocation
	}

	//
	// sodium_malloc() already attempts mlock(), but libsodium
	// intentionally doesn't fail allocation if mlock fails.
	//
	// Sigryx requires locked secret memory, so we explicitly
	// verify it and fail closed.
	//
	if C.sodium_mlock(ptr, C.size_t(len(data))) != 0 {
		C.sodium_free(ptr)
		return nil, ErrMemoryLock
	}

	C.memcpy(
		ptr,
		unsafe.Pointer(&data[0]),
		C.size_t(len(data)),
	)

	if C.sodium_mprotect_noaccess(ptr) != 0 {
		C.sodium_free(ptr)
		return nil, ErrMemoryProtection
	}

	return &region{
		ptr:  ptr,
		size: len(data),
	}, nil
}

func newRandomRegion(size int) (*region, error) {
	if size <= 0 {
		return nil, ErrInvalidSize
	}

	if err := initializeSodium(); err != nil {
		return nil, err
	}

	ptr := C.sodium_malloc(C.size_t(size))
	if ptr == nil {
		return nil, ErrAllocation
	}

	if C.sodium_mlock(ptr, C.size_t(size)) != 0 {
		C.sodium_free(ptr)
		return nil, ErrMemoryLock
	}

	//
	// Random material is generated directly inside secure memory.
	// No plaintext Go heap copy is created.
	//
	C.randombytes_buf(
		ptr,
		C.size_t(size),
	)

	if C.sodium_mprotect_noaccess(ptr) != 0 {
		C.sodium_free(ptr)
		return nil, ErrMemoryProtection
	}

	return &region{
		ptr:  ptr,
		size: size,
	}, nil
}

func (r *region) withBytes(
	fn func([]byte) error,
) (err error) {
	if r == nil || r.ptr == nil {
		return ErrDestroyed
	}

	//
	// Secret is normally NOACCESS.
	//
	// For the lifetime of the callback we switch it to READONLY.
	// Accidental writes will therefore terminate the process
	// instead of silently mutating secret material.
	//
	if C.sodium_mprotect_readonly(r.ptr) != 0 {
		return ErrMemoryProtection
	}

	defer func() {
		if C.sodium_mprotect_noaccess(r.ptr) != 0 {
			err = errors.Join(
				err,
				ErrMemoryProtection,
			)
		}

		runtime.KeepAlive(r)
	}()

	data := unsafe.Slice(
		(*byte)(r.ptr),
		r.size,
	)

	return fn(data)
}

func (r *region) destroy() {
	if r == nil || r.ptr == nil {
		return
	}

	//
	// sodium_free():
	//
	// - checks the guarded allocation canary
	// - changes memory protection when required
	// - zeroes the memory
	// - unlocks it
	// - releases the allocation
	//
	C.sodium_free(r.ptr)

	r.ptr = nil
	r.size = 0
}

func wipeGoBytes(data []byte) {
	if len(data) == 0 {
		return
	}

	C.sodium_memzero(
		unsafe.Pointer(&data[0]),
		C.size_t(len(data)),
	)

	runtime.KeepAlive(data)
}
