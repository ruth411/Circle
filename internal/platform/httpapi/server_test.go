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

	"github.com/ruth411/circle/internal/diner"
	"github.com/ruth411/circle/internal/tenancy"
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

func TestHandlerGenerated404IsPreserved(t *testing.T) {
	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		DinerService: diner.NewService(),
	})
	req := httptest.NewRequest(http.MethodGet, "/diner/tokens/missing-token", nil)
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}

	var payload ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Error.Code != "token_unavailable" {
		t.Fatalf("code = %q, want token_unavailable", payload.Error.Code)
	}
}

func TestMethodMismatchReturnsServeMux405(t *testing.T) {
	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		OrderingService:      seedOrderingService(),
		SessionValidator:     seedSessionService(t, "loc-1"),
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})
	req := httptest.NewRequest(http.MethodGet, "/orders", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405, body = %s", recorder.Code, recorder.Body.String())
	}
	if allow := recorder.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("Allow = %q, want %q", allow, http.MethodPost)
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

func TestDinerTokensAreRedactedInLogs(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	server := NewServer(logger)

	req := httptest.NewRequest(http.MethodGet, "/diner/tokens/token-secret-1", nil)
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	logOutput := logBuffer.String()
	if strings.Contains(logOutput, "token-secret-1") {
		t.Fatalf("log output leaked token, got %q", logOutput)
	}
	if !strings.Contains(logOutput, `"/diner/tokens/{token}"`) {
		t.Fatalf("log output missing redacted path, got %q", logOutput)
	}
}
