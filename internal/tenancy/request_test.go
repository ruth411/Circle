package tenancy

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
)

func TestHeaderResolverReadsLocationID(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Location-Id", "loc-1")

	locationID, err := HeaderResolver{}.Resolve(req)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if locationID != "loc-1" {
		t.Fatalf("locationID = %q, want loc-1", locationID)
	}
}

func TestHeaderResolverRejectsMissingLocationID(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)

	_, err := HeaderResolver{}.Resolve(req)
	if !errors.Is(err, ErrLocationRequired) {
		t.Fatalf("err = %v, want ErrLocationRequired", err)
	}
}

func TestWithLocationIDStoresValueInContext(t *testing.T) {
	ctx := WithLocationID(context.Background(), "loc-1")

	locationID, ok := LocationID(ctx)
	if !ok {
		t.Fatal("expected location id in context")
	}
	if locationID != "loc-1" {
		t.Fatalf("locationID = %q, want loc-1", locationID)
	}
}

func TestStaticOrganizationResolverReturnsOrganization(t *testing.T) {
	organizationID, err := StaticOrganizationResolver{
		"loc-1": "org-1",
	}.OrganizationIDForLocation(context.Background(), "loc-1")
	if err != nil {
		t.Fatalf("OrganizationIDForLocation returned error: %v", err)
	}
	if organizationID != "org-1" {
		t.Fatalf("organizationID = %q, want org-1", organizationID)
	}
}
