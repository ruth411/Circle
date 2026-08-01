package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/core/recipe"
	"github.com/ruth411/circle/internal/diner"
	"github.com/ruth411/circle/internal/ordering"
	"github.com/ruth411/circle/internal/purchasing"
	"github.com/ruth411/circle/internal/tenancy"
)

type contextKey string

const requestIDKey contextKey = "request_id"

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

func NewServer(logger *slog.Logger) http.Handler {
	return NewServerWithDependencies(logger, Dependencies{})
}

type Dependencies struct {
	IngredientService    *ingredient.Service
	RecipeService        *recipe.Service
	CatalogService       *recipe.CatalogService
	DinerService         *diner.Service
	OrderingService      *ordering.Service
	PurchasingService    *purchasing.Service
	SessionValidator     SessionValidator
	LocationResolver     tenancy.Resolver
	OrganizationResolver tenancy.OrganizationResolver
}

func NewServerWithDependencies(logger *slog.Logger, deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthz)
	mux.HandleFunc("/readyz", healthz)
	registerIngredientRoutes(mux, ingredientDependencies{
		service:              deps.IngredientService,
		locationResolver:     deps.LocationResolver,
		organizationResolver: deps.OrganizationResolver,
		sessionValidator:     deps.SessionValidator,
	})
	registerRecipeRoutes(mux, recipeDependencies{
		service:              deps.RecipeService,
		locationResolver:     deps.LocationResolver,
		organizationResolver: deps.OrganizationResolver,
		sessionValidator:     deps.SessionValidator,
	})
	registerCatalogRoutes(mux, catalogDependencies{
		service:              deps.CatalogService,
		locationResolver:     deps.LocationResolver,
		organizationResolver: deps.OrganizationResolver,
		sessionValidator:     deps.SessionValidator,
	})
	registerOrderingRoutes(mux, orderingDependencies{
		service:              deps.OrderingService,
		locationResolver:     deps.LocationResolver,
		organizationResolver: deps.OrganizationResolver,
		sessionValidator:     deps.SessionValidator,
	})
	registerPurchasingRoutes(mux, purchasingDependencies{
		service:              deps.PurchasingService,
		locationResolver:     deps.LocationResolver,
		organizationResolver: deps.OrganizationResolver,
		sessionValidator:     deps.SessionValidator,
	})
	registerDinerRoutes(mux, dinerDependencies{
		service: deps.DinerService,
	})
	return withRecover(logger, withRequestID(withLogging(logger, withJSONNotFound(mux))))
}

func healthz(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]any{
		"service":    "circle",
		"status":     "ok",
		"request_id": RequestID(r.Context()),
		"time":       time.Now().UTC().Format(time.RFC3339),
	})
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code string, message string) {
	WriteJSON(w, status, ErrorResponse{
		Error: ErrorBody{
			Code:      code,
			Message:   message,
			RequestID: RequestID(r.Context()),
		},
	})
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = newRequestID()
		}

		w.Header().Set("X-Request-Id", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		logger.Info("http request",
			"method", r.Method,
			"path", loggedPath(r.URL.Path),
			"status", recorder.status,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"request_id", RequestID(r.Context()),
		)
	})
}

func withRecover(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered", "panic", recovered, "request_id", RequestID(r.Context()))
				WriteError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func withJSONNotFound(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &bufferedResponseWriter{
			header: make(http.Header),
			status: http.StatusOK,
		}
		next.ServeHTTP(recorder, r)
		if recorder.status == http.StatusNotFound && servesMuxNotFound(recorder.body.Bytes(), recorder.header) {
			WriteError(w, r, http.StatusNotFound, "not_found", "route not found")
			return
		}

		copyHeaders(w.Header(), recorder.header)
		w.WriteHeader(recorder.status)
		_, _ = w.Write(recorder.body.Bytes())
	})
}

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func newRequestID() string {
	raw := make([]byte, 9)
	if _, err := rand.Read(raw); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}

	return base64.RawURLEncoding.EncodeToString(raw)
}

func copyHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func loggedPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/diner/tokens/"):
		return "/diner/tokens/{token}"
	case strings.HasPrefix(path, "/diner/claims/"):
		return "/diner/claims/{id}"
	default:
		return path
	}
}

func servesMuxNotFound(body []byte, header http.Header) bool {
	if len(body) == 0 {
		return true
	}
	if contentType := header.Get("Content-Type"); contentType != "" && !strings.HasPrefix(contentType, "text/plain") {
		return false
	}
	return string(body) == "404 page not found\n"
}

type bufferedResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *bufferedResponseWriter) Header() http.Header {
	return w.header
}

func (w *bufferedResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *bufferedResponseWriter) Write(p []byte) (int, error) {
	return w.body.Write(p)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func IsBrokenPipe(err error) bool {
	return errors.Is(err, http.ErrAbortHandler)
}
