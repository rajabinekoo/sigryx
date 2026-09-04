package alert

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rajabinekoo/sigryx/internal/core/domain"
)

func TestWebhookSendsCriticalAlert(t *testing.T) {
	var received domain.Alert
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sink, err := NewWebhook(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Send(context.Background(), domain.Alert{
		Severity: "CRITICAL", Code: "INTEGRITY_VALUE_MISMATCH", Context: "ledger:v1", ObjectID: "j-1",
	}); err != nil {
		t.Fatal(err)
	}
	if received.Code != "INTEGRITY_VALUE_MISMATCH" || received.ObjectID != "j-1" {
		t.Fatalf("unexpected alert: %+v", received)
	}
}

func TestWebhookDisabledWhenURLIsEmpty(t *testing.T) {
	sink, err := NewWebhook("", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Send(context.Background(), domain.Alert{Code: "X"}); err != nil {
		t.Fatal(err)
	}
}

func TestWebhookRejectsInvalidURL(t *testing.T) {
	if _, err := NewWebhook("not-a-url", time.Second); err == nil {
		t.Fatal("expected invalid URL error")
	}
}
