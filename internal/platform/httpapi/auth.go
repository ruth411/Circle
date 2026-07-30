package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/ruth411/circle/internal/identity"
	"github.com/ruth411/circle/internal/tenancy"
)

const sessionIDHeader = "X-Session-Id"

type SessionValidator interface {
	ValidateSession(context.Context, string) (identity.Session, error)
}

func WithResolvedLocation(resolver tenancy.Resolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, err := resolver.Resolve(r)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "location_required", "location id is required")
			return
		}

		next.ServeHTTP(w, r.WithContext(tenancy.WithLocationID(r.Context(), locationID)))
	})
}

func RequireStaffSession(validator SessionValidator, organizationResolver tenancy.OrganizationResolver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := strings.TrimSpace(r.Header.Get(sessionIDHeader))
		if sessionID == "" {
			WriteError(w, r, http.StatusUnauthorized, "session_required", "staff session is required")
			return
		}

		session, err := validator.ValidateSession(r.Context(), sessionID)
		if err != nil {
			status := http.StatusUnauthorized
			code := "invalid_session"
			message := "staff session is invalid"
			if errors.Is(err, identity.ErrLocationMismatch) {
				status = http.StatusForbidden
				code = "location_mismatch"
				message = "session does not match requested location"
			}
			WriteError(w, r, status, code, message)
			return
		}

		locationID, ok := tenancy.LocationID(r.Context())
		if ok {
			if organizationResolver == nil {
				WriteError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
				return
			}

			organizationID, err := organizationResolver.OrganizationIDForLocation(r.Context(), locationID)
			if err != nil {
				status := http.StatusInternalServerError
				code := "internal_error"
				message := "internal server error"
				if errors.Is(err, tenancy.ErrLocationNotFound) {
					status = http.StatusBadRequest
					code = "location_not_found"
					message = "requested location was not found"
				}
				WriteError(w, r, status, code, message)
				return
			}

			if err := identity.AuthorizeLocationAccess(session, locationID, organizationID); err != nil {
				code := "location_mismatch"
				message := "session does not match requested location"
				if errors.Is(err, identity.ErrOrganizationMismatch) {
					code = "organization_mismatch"
					message = "session does not match requested organization"
				}
				WriteError(w, r, http.StatusForbidden, code, message)
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(identity.WithSession(r.Context(), session)))
	})
}
