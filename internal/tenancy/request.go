package tenancy

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

var ErrLocationRequired = errors.New("location id required")
var ErrLocationNotFound = errors.New("location not found")

type contextKey string

const locationIDKey contextKey = "tenancy.location_id"

type Resolver interface {
	Resolve(*http.Request) (string, error)
}

type OrganizationResolver interface {
	OrganizationIDForLocation(context.Context, string) (string, error)
}

type HeaderResolver struct {
	Header string
}

func (r HeaderResolver) Resolve(req *http.Request) (string, error) {
	header := r.Header
	if header == "" {
		header = "X-Location-Id"
	}

	locationID := strings.TrimSpace(req.Header.Get(header))
	if locationID == "" {
		return "", ErrLocationRequired
	}

	return locationID, nil
}

func WithLocationID(ctx context.Context, locationID string) context.Context {
	return context.WithValue(ctx, locationIDKey, locationID)
}

func LocationID(ctx context.Context) (string, bool) {
	locationID, ok := ctx.Value(locationIDKey).(string)
	return locationID, ok && locationID != ""
}

type StaticOrganizationResolver map[string]string

func (r StaticOrganizationResolver) OrganizationIDForLocation(_ context.Context, locationID string) (string, error) {
	organizationID, ok := r[locationID]
	if !ok || strings.TrimSpace(organizationID) == "" {
		return "", ErrLocationNotFound
	}
	return organizationID, nil
}
