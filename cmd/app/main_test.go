package main

import (
	"net/http/httptest"
	"testing"
)

func TestSendJSON_SetsContentTypeAndBody(t *testing.T) {
	rec := httptest.NewRecorder()

	sendJSON(rec, map[string]string{"hello": "world"})

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: ждали application/json, получили %q", got)
	}
	want := `{"hello":"world"}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("тело: ждали %q, получили %q", want, got)
	}
}