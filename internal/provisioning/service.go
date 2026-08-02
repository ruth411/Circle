package provisioning

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/ruth411/circle/internal/identity"
)

var (
	ErrInvalidProvisioning  = errors.New("invalid provisioning")
	ErrProvisioningConflict = errors.New("provisioning conflict")
)

type ProvisionLocationInput struct {
	OrganizationID   string
	OrganizationName string
	RestaurantID     string
	RestaurantName   string
	LocationID       string
	LocationName     string
	TimezoneName     string
	Currency         string
	RoleID           string
	RoleName         string
	UserID           string
	Email            string
	DisplayName      string
	PasswordHash     string
	SessionID        string
	SessionTTL       time.Duration
}

type ProvisionLocationParams struct {
	OrganizationID   string
	OrganizationName string
	RestaurantID     string
	RestaurantName   string
	LocationID       string
	LocationName     string
	TimezoneName     string
	Currency         string
	RoleID           string
	RoleName         string
	UserID           string
	Email            string
	DisplayName      string
	PasswordHash     string
	SessionID        string
	ScopeType        identity.ScopeType
	SessionExpiresAt time.Time
}

type ProvisionedLocation struct {
	OrganizationID string
	RestaurantID   string
	LocationID     string
	RoleID         string
	UserID         string
	Session        identity.Session
}

type Repository interface {
	ProvisionLocation(context.Context, ProvisionLocationParams) (ProvisionedLocation, error)
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{repo: repo, now: now}
}

func (s *Service) ProvisionLocation(ctx context.Context, input ProvisionLocationInput) (ProvisionedLocation, error) {
	if s.repo == nil {
		return ProvisionedLocation{}, fmt.Errorf("provisioning repository is required")
	}

	params, err := normalizeProvisionLocation(input, s.now())
	if err != nil {
		return ProvisionedLocation{}, err
	}
	return s.repo.ProvisionLocation(ctx, params)
}

func normalizeProvisionLocation(input ProvisionLocationInput, now time.Time) (ProvisionLocationParams, error) {
	params := ProvisionLocationParams{
		OrganizationID:   strings.TrimSpace(input.OrganizationID),
		OrganizationName: strings.TrimSpace(input.OrganizationName),
		RestaurantID:     strings.TrimSpace(input.RestaurantID),
		RestaurantName:   strings.TrimSpace(input.RestaurantName),
		LocationID:       strings.TrimSpace(input.LocationID),
		LocationName:     strings.TrimSpace(input.LocationName),
		TimezoneName:     strings.TrimSpace(input.TimezoneName),
		Currency:         strings.ToUpper(strings.TrimSpace(input.Currency)),
		RoleID:           strings.TrimSpace(input.RoleID),
		RoleName:         strings.TrimSpace(input.RoleName),
		UserID:           strings.TrimSpace(input.UserID),
		Email:            strings.TrimSpace(input.Email),
		DisplayName:      strings.TrimSpace(input.DisplayName),
		PasswordHash:     strings.TrimSpace(input.PasswordHash),
		SessionID:        strings.TrimSpace(input.SessionID),
		ScopeType:        identity.ScopeTypeLocation,
	}

	switch {
	case params.OrganizationID == "":
		return ProvisionLocationParams{}, fmt.Errorf("%w: organization id is required", ErrInvalidProvisioning)
	case params.OrganizationName == "":
		return ProvisionLocationParams{}, fmt.Errorf("%w: organization name is required", ErrInvalidProvisioning)
	case params.RestaurantID == "":
		return ProvisionLocationParams{}, fmt.Errorf("%w: restaurant id is required", ErrInvalidProvisioning)
	case params.RestaurantName == "":
		return ProvisionLocationParams{}, fmt.Errorf("%w: restaurant name is required", ErrInvalidProvisioning)
	case params.LocationID == "":
		return ProvisionLocationParams{}, fmt.Errorf("%w: location id is required", ErrInvalidProvisioning)
	case params.LocationName == "":
		return ProvisionLocationParams{}, fmt.Errorf("%w: location name is required", ErrInvalidProvisioning)
	case params.TimezoneName == "":
		return ProvisionLocationParams{}, fmt.Errorf("%w: timezone name is required", ErrInvalidProvisioning)
	case params.Currency == "":
		return ProvisionLocationParams{}, fmt.Errorf("%w: currency is required", ErrInvalidProvisioning)
	case params.RoleID == "":
		return ProvisionLocationParams{}, fmt.Errorf("%w: role id is required", ErrInvalidProvisioning)
	case params.RoleName == "":
		return ProvisionLocationParams{}, fmt.Errorf("%w: role name is required", ErrInvalidProvisioning)
	case params.UserID == "":
		return ProvisionLocationParams{}, fmt.Errorf("%w: user id is required", ErrInvalidProvisioning)
	case params.Email == "":
		return ProvisionLocationParams{}, fmt.Errorf("%w: email is required", ErrInvalidProvisioning)
	case params.DisplayName == "":
		return ProvisionLocationParams{}, fmt.Errorf("%w: display name is required", ErrInvalidProvisioning)
	case params.PasswordHash == "":
		return ProvisionLocationParams{}, fmt.Errorf("%w: password hash is required", ErrInvalidProvisioning)
	case params.SessionID == "":
		return ProvisionLocationParams{}, fmt.Errorf("%w: session id is required", ErrInvalidProvisioning)
	case input.SessionTTL <= 0:
		return ProvisionLocationParams{}, fmt.Errorf("%w: session ttl must be positive", ErrInvalidProvisioning)
	}

	if _, err := mail.ParseAddress(params.Email); err != nil {
		return ProvisionLocationParams{}, fmt.Errorf("%w: email is invalid", ErrInvalidProvisioning)
	}
	if _, err := time.LoadLocation(params.TimezoneName); err != nil {
		return ProvisionLocationParams{}, fmt.Errorf("%w: timezone name is invalid", ErrInvalidProvisioning)
	}

	params.SessionExpiresAt = now.UTC().Add(input.SessionTTL)
	return params, nil
}
