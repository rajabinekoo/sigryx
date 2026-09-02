package canonicaljson

import (
	"bytes"
	"testing"
)

func TestMarshalIsStableAcrossObjectOrderAndNumberForms(t *testing.T) {
	a, err := Marshal([]byte(`{"b":1.0,"a":{"y":1e0,"x":true}}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Marshal([]byte(` { "a" : {"x":true,"y":1}, "b":1 } `))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("canonical JSON differs:\n%s\n%s", a, b)
	}
	if string(a) != `{"a":{"x":true,"y":1},"b":1}` {
		t.Fatalf("canonical JSON = %s", a)
	}
}

func TestMarshalRejectsDuplicateKeys(t *testing.T) {
	if _, err := Marshal([]byte(`{"a":1,"a":2}`)); err == nil {
		t.Fatal("expected duplicate key error")
	}
}
