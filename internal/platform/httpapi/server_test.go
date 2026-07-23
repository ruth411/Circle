package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthzReturnsJSONAndRequestID(t *testing.T) {
	server := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "req-123")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("X-Request-Id"); got != "req-123" {
		t.Fatalf("X-Request-Id = %q, want req-123", got)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	if payload["service"] != "circle" {
		t.Fatalf("service = %v, want circle", payload["service"])
	}
	if payload["request_id"] != "req-123" {
		t.Fatalf("request_id = %v, want req-123", payload["request_id"])
	}
}

func TestUnknownRouteReturnsJSONError(t *testing.T) {
	server := NewServer(slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}

	var payload ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Error.Code != "not_found" {
		t.Fatalf("code = %q, want not_found", payload.Error.Code)
	}
	if payload.Error.RequestID == "" {
		t.Fatal("request_id empty, want generated id")
	}
}

func TestRequestIDIsPresentInLogs(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	server := NewServer(logger)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "req-log-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	logOutput := logBuffer.String()
	if !strings.Contains(logOutput, `"request_id":"req-log-1"`) {
		t.Fatalf("log output missing request_id, got %q", logOutput)
	}
}
