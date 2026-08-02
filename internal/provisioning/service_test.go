package provisioning

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ruth411/circle/internal/identity"
)

type stubRepository struct {
	provisionFn func(context.Context, ProvisionLocationParams) (ProvisionedLocation, error)
}

func (s stubRepository) ProvisionLocation(ctx context.Context, params ProvisionLocationParams) (ProvisionedLocation, error) {
	return s.provisionFn(ctx, params)
}

func TestServiceProvisionLocationRejectsMissingFields(t *testing.T) {
	service := NewService(stubRepository{
		provisionFn: func(context.Context, ProvisionLocationParams) (ProvisionedLocation, error) {
			t.Fatal("repo should not be called")
			return ProvisionedLocation{}, nil
		},
	}, func() time.Time {
		return time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	})

	_, err := service.ProvisionLocation(context.Background(), ProvisionLocationInput{
		OrganizationID:   "org-1",
		OrganizationName: "Org 1",
	})
	if !errors.Is(err, ErrInvalidProvisioning) {
		t.Fatalf("error = %v, want ErrInvalidProvisioning", err)
	}
}

func TestServiceProvisionLocationNormalizesAndDelegates(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	service := NewService(stubRepository{
		provisionFn: func(_ context.Context, params ProvisionLocationParams) (ProvisionedLocation, error) {
			if params.OrganizationID != "org-1" {
				t.Fatalf("organization id = %q, want org-1", params.OrganizationID)
			}
			if params.LocationID != "loc-1" {
				t.Fatalf("location id = %q, want loc-1", params.LocationID)
			}
			if params.RoleName != "store_manager" {
				t.Fatalf("role name = %q, want store_manager", params.RoleName)
			}
			if params.ScopeType != identity.ScopeTypeLocation {
				t.Fatalf("scope type = %q, want %q", params.ScopeType, identity.ScopeTypeLocation)
			}
			if got := params.SessionExpiresAt; !got.Equal(now.Add(24 * time.Hour)) {
				t.Fatalf("session expires at = %s, want %s", got, now.Add(24*time.Hour))
			}
			return ProvisionedLocation{
				OrganizationID: params.OrganizationID,
				RestaurantID:   params.RestaurantID,
				LocationID:     params.LocationID,
				RoleID:         params.RoleID,
				UserID:         params.UserID,
				Session: identity.Session{
					ID:             params.SessionID,
					UserID:         params.UserID,
					OrganizationID: params.OrganizationID,
					ScopeType:      params.ScopeType,
					LocationID:     params.LocationID,
					ExpiresAt:      params.SessionExpiresAt,
				},
			}, nil
		},
	}, func() time.Time { return now })

	provisioned, err := service.ProvisionLocation(context.Background(), ProvisionLocationInput{
		OrganizationID:   " org-1 ",
		OrganizationName: " Org 1 ",
		RestaurantID:     " rest-1 ",
		RestaurantName:   " Main Restaurant ",
		LocationID:       " loc-1 ",
		LocationName:     " Uptown ",
		TimezoneName:     " America/New_York ",
		Currency:         " usd ",
		RoleID:           " role-1 ",
		RoleName:         " store_manager ",
		UserID:           " user-1 ",
		Email:            " manager@example.com ",
		DisplayName:      " Manager ",
		PasswordHash:     " hash ",
		SessionID:        " session-1 ",
		SessionTTL:       24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("ProvisionLocation returned error: %v", err)
	}
	if provisioned.LocationID != "loc-1" {
		t.Fatalf("location id = %q, want loc-1", provisioned.LocationID)
	}
	if provisioned.Session.ScopeType != identity.ScopeTypeLocation {
		t.Fatalf("session scope = %q, want %q", provisioned.Session.ScopeType, identity.ScopeTypeLocation)
	}
}
