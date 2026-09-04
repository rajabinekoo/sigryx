package canonicaljson

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

func TestSelectSupportsNestedObjectsArraysAndPointerEscapes(t *testing.T) {
	payload := []byte(`{
		"journal":{"id":"j-1","lines":[{"amount":1.0},{"amount":2}]},
		"a/b":{"~value":true},
		"ignored":"changes-are-allowed"
	}`)

	selected, fields, err := Select(payload, []string{
		"/journal/lines/1/amount",
		"/a~1b/~0value",
		"/journal/id",
	})
	if err != nil {
		t.Fatal(err)
	}

	expected := []byte(`{"/a~1b/~0value":true,"/journal/id":"j-1","/journal/lines/1/amount":2}`)
	if !bytes.Equal(selected, expected) {
		t.Fatalf("selected = %s", selected)
	}
	if !reflect.DeepEqual(fields, []string{"/a~1b/~0value", "/journal/id", "/journal/lines/1/amount"}) {
		t.Fatalf("fields = %#v", fields)
	}
}

func TestSelectIsStableAcrossFieldOrderAndNumberForms(t *testing.T) {
	a, fieldsA, err := Select([]byte(`{"x":{"b":1.0,"a":1e0}}`), []string{"/x/b", "/x/a"})
	if err != nil {
		t.Fatal(err)
	}
	b, fieldsB, err := Select([]byte(`{"x":{"a":1,"b":1}}`), []string{"/x/a", "/x/b"})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) || !reflect.DeepEqual(fieldsA, fieldsB) {
		t.Fatalf("selection is not canonical: %s / %s", a, b)
	}
}

func TestSelectRejectsMissingAndDuplicatePointers(t *testing.T) {
	_, _, err := Select([]byte(`{"a":1}`), []string{"/missing"})
	if !errors.Is(err, ErrPointerNotFound) {
		t.Fatalf("expected ErrPointerNotFound, got %v", err)
	}
	_, _, err = Select([]byte(`{"a":1}`), []string{"/a", "/a"})
	if !errors.Is(err, ErrDuplicatePointer) {
		t.Fatalf("expected ErrDuplicatePointer, got %v", err)
	}
}
