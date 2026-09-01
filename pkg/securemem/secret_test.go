package securemem

import (
	"bytes"
	"errors"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	source := []byte("super-secret-value")

	expected := append(
		[]byte(nil),
		source...,
	)

	secret, err := New(source)
	if err != nil {
		t.Fatal(err)
	}
	defer secret.Destroy()

	if !bytes.Equal(
		source,
		make([]byte, len(source)),
	) {
		t.Fatal("source was not wiped")
	}

	err = secret.WithBytes(func(data []byte) error {
		if !bytes.Equal(data, expected) {
			t.Fatalf(
				"unexpected secret: %q",
				data,
			)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRandom(t *testing.T) {
	a, err := Random(32)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Destroy()

	b, err := Random(32)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Destroy()

	var aCopy []byte

	err = a.WithBytes(func(data []byte) error {
		aCopy = append(
			[]byte(nil),
			data...,
		)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	err = b.WithBytes(func(data []byte) error {
		if bytes.Equal(aCopy, data) {
			t.Fatal("random secrets are equal")
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRandomSize(t *testing.T) {
	secret, err := Random(32)
	if err != nil {
		t.Fatal(err)
	}
	defer secret.Destroy()

	if secret.Size() != 32 {
		t.Fatalf(
			"expected size 32, got %d",
			secret.Size(),
		)
	}
}

func TestRandomInvalidSize(t *testing.T) {
	_, err := Random(0)

	if !errors.Is(err, ErrInvalidSize) {
		t.Fatalf(
			"expected ErrInvalidSize, got %v",
			err,
		)
	}
}

func TestDestroy(t *testing.T) {
	secret, err := Random(32)
	if err != nil {
		t.Fatal(err)
	}

	secret.Destroy()

	if !secret.IsDestroyed() {
		t.Fatal("secret should be destroyed")
	}

	if secret.Size() != 0 {
		t.Fatal("destroyed secret should have zero size")
	}

	err = secret.WithBytes(func([]byte) error {
		return nil
	})

	if !errors.Is(err, ErrDestroyed) {
		t.Fatalf(
			"expected ErrDestroyed, got %v",
			err,
		)
	}
}

func TestDestroyTwice(t *testing.T) {
	secret, err := Random(32)
	if err != nil {
		t.Fatal(err)
	}

	secret.Destroy()
	secret.Destroy()
}

func TestCallbackError(t *testing.T) {
	secret, err := Random(32)
	if err != nil {
		t.Fatal(err)
	}
	defer secret.Destroy()

	expected := errors.New("callback failed")

	err = secret.WithBytes(func([]byte) error {
		return expected
	})

	if !errors.Is(err, expected) {
		t.Fatalf(
			"expected callback error, got %v",
			err,
		)
	}
}

func TestConcurrentAccess(t *testing.T) {
	secret, err := Random(32)
	if err != nil {
		t.Fatal(err)
	}
	defer secret.Destroy()

	const goroutines = 32

	var wg sync.WaitGroup

	for range goroutines {
		wg.Add(1)

		go func() {
			defer wg.Done()

			err := secret.WithBytes(
				func(data []byte) error {
					if len(data) != 32 {
						t.Errorf(
							"unexpected size: %d",
							len(data),
						)
					}

					return nil
				},
			)

			if err != nil {
				t.Errorf(
					"with bytes: %v",
					err,
				)
			}
		}()
	}

	wg.Wait()
}
