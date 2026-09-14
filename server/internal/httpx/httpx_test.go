package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type decodeFixture struct {
	Name string `json:"name"`
}

func TestDecodeJSONAcceptsValidBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"cell"}`))
	w := httptest.NewRecorder()
	var got decodeFixture

	if !DecodeJSONLimit(w, r, &got, 1024) {
		t.Fatalf("valid JSON should decode, status=%d body=%s", w.Code, w.Body.String())
	}
	if got.Name != "cell" {
		t.Fatalf("unexpected decoded value: %+v", got)
	}
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"cell","admin":true}`))
	w := httptest.NewRecorder()
	var got decodeFixture

	if DecodeJSONLimit(w, r, &got, 1024) {
		t.Fatal("unknown field should be rejected")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDecodeJSONRejectsTrailingDocument(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"cell"} {"name":"other"}`))
	w := httptest.NewRecorder()
	var got decodeFixture

	if DecodeJSONLimit(w, r, &got, 1024) {
		t.Fatal("second JSON document should be rejected")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDecodeJSONReturns413WhenBodyTooLarge(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"0123456789"}`))
	w := httptest.NewRecorder()
	var got decodeFixture

	if DecodeJSONLimit(w, r, &got, 8) {
		t.Fatal("oversized body should be rejected")
	}
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d body=%s", w.Code, w.Body.String())
	}
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("error response should stay JSON: %v", err)
	}
	if resp.Code != 50001 {
		t.Fatalf("unexpected business code: %d", resp.Code)
	}
}
