package diner

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
)

const (
	TokenTTL            = 24 * time.Hour
	NutritionDisclaimer = "Nutrition values are estimates."
	outboxConsumer      = "diner"
)

var (
	ErrInvalidTokenData   = errors.New("invalid receipt token data")
	ErrTokenNotFound      = errors.New("receipt token not found")
	ErrTokenExpired       = errors.New("receipt token expired")
	ErrTokenAlreadyExists = errors.New("receipt token already exists")
	ErrClaimAlreadyExists = errors.New("claim already exists")
	ErrClaimNotFound      = errors.New("claim not found")
	ErrInvalidClaim       = errors.New("invalid claim")
)

type PublicOrderItem struct {
	ItemID string
	LineID string
	Name   string
	Macros ingredient.MacroValues
}

type ReceiptToken struct {
	Token      string
	OrderID    string
	LocationID string
	ClosedAt   time.Time
	ExpiresAt  time.Time
	Items      []PublicOrderItem
}

type Claim struct {
	ID              string
	Token           string
	LocationID      string
	SelectedItemIDs []string
	Totals          ingredient.MacroValues
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Repository interface {
	GetToken(context.Context, string) (ReceiptToken, error)
	GetTokenByOrder(context.Context, string, string) (ReceiptToken, error)
	CreateToken(context.Context, ReceiptToken) error
	GetClaim(context.Context, string) (Claim, error)
	CreateClaim(context.Context, Claim) error
	UpdateClaim(context.Context, Claim) error
}

type Service struct {
	repo Repository
	now  func() time.Time
	rand io.Reader
}

func NewService() *Service {
	return NewServiceWithRepository(newMemoryRepository())
}

func NewServiceWithRepository(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
		rand: rand.Reader,
	}
}

func (s *Service) SetNowForTests(now func() time.Time) {
	s.now = now
}

func (s *Service) IssueToken(ctx context.Context, order contracts.ClosedOrder) (ReceiptToken, error) {
	if s.repo == nil {
		return ReceiptToken{}, fmt.Errorf("diner repository is required")
	}

	orderID := strings.TrimSpace(order.OrderID)
	locationID := strings.TrimSpace(order.LocationID)
	if orderID == "" {
		return ReceiptToken{}, fmt.Errorf("%w: closed order id is required", ErrInvalidTokenData)
	}
	if locationID == "" {
		return ReceiptToken{}, fmt.Errorf("%w: closed order location id is required", ErrInvalidTokenData)
	}
	if order.ClosedAt.IsZero() {
		return ReceiptToken{}, fmt.Errorf("%w: closed order timestamp is required", ErrInvalidTokenData)
	}

	existing, err := s.repo.GetTokenByOrder(ctx, locationID, orderID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrTokenNotFound) {
		return ReceiptToken{}, err
	}

	tokenValue, err := s.newOpaqueID(18)
	if err != nil {
		return ReceiptToken{}, err
	}

	token := ReceiptToken{
		Token:      tokenValue,
		OrderID:    orderID,
		LocationID: locationID,
		ClosedAt:   order.ClosedAt.UTC(),
		ExpiresAt:  order.ClosedAt.UTC().Add(TokenTTL),
	}
	items, err := expandClosedOrderItems(order)
	if err != nil {
		return ReceiptToken{}, err
	}
	token.Items = items
	if err := s.repo.CreateToken(ctx, token); err != nil {
		if errors.Is(err, ErrTokenAlreadyExists) {
			return s.repo.GetTokenByOrder(ctx, locationID, orderID)
		}
		return ReceiptToken{}, err
	}
	return cloneToken(token), nil
}

func (s *Service) ResolveToken(ctx context.Context, token string) (ReceiptToken, error) {
	if s.repo == nil {
		return ReceiptToken{}, fmt.Errorf("diner repository is required")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return ReceiptToken{}, ErrTokenNotFound
	}

	stored, err := s.repo.GetToken(ctx, token)
	if err != nil {
		return ReceiptToken{}, err
	}
	if tokenExpired(s.now(), stored.ExpiresAt) {
		return ReceiptToken{}, ErrTokenExpired
	}
	return cloneToken(stored), nil
}

func (s *Service) ResolveTokenByOrder(ctx context.Context, locationID string, orderID string) (ReceiptToken, error) {
	if s.repo == nil {
		return ReceiptToken{}, fmt.Errorf("diner repository is required")
	}
	locationID = strings.TrimSpace(locationID)
	orderID = strings.TrimSpace(orderID)
	if locationID == "" || orderID == "" {
		return ReceiptToken{}, ErrTokenNotFound
	}

	stored, err := s.repo.GetTokenByOrder(ctx, locationID, orderID)
	if err != nil {
		return ReceiptToken{}, err
	}
	if tokenExpired(s.now(), stored.ExpiresAt) {
		return ReceiptToken{}, ErrTokenExpired
	}
	return cloneToken(stored), nil
}

func (s *Service) SubmitClaim(ctx context.Context, claimID string, token string, selectedItemIDs []string) (Claim, error) {
	if s.repo == nil {
		return Claim{}, fmt.Errorf("diner repository is required")
	}
	if strings.TrimSpace(claimID) == "" {
		var err error
		claimID, err = s.newOpaqueID(12)
		if err != nil {
			return Claim{}, err
		}
	}

	claim, err := s.buildClaim(ctx, claimID, token, selectedItemIDs)
	if err != nil {
		return Claim{}, err
	}
	if err := s.repo.CreateClaim(ctx, claim); err != nil {
		return Claim{}, err
	}
	return cloneClaim(claim), nil
}

func (s *Service) ReviseClaim(ctx context.Context, claimID string, token string, selectedItemIDs []string) (Claim, error) {
	if s.repo == nil {
		return Claim{}, fmt.Errorf("diner repository is required")
	}
	claimID = strings.TrimSpace(claimID)
	if claimID == "" {
		return Claim{}, fmt.Errorf("%w: claim id is required", ErrInvalidClaim)
	}

	existing, err := s.repo.GetClaim(ctx, claimID)
	if err != nil {
		return Claim{}, err
	}
	if existing.Token != strings.TrimSpace(token) {
		return Claim{}, fmt.Errorf("%w: claim belongs to a different token", ErrInvalidClaim)
	}

	claim, err := s.buildClaim(ctx, claimID, token, selectedItemIDs)
	if err != nil {
		return Claim{}, err
	}
	claim.CreatedAt = existing.CreatedAt
	if err := s.repo.UpdateClaim(ctx, claim); err != nil {
		return Claim{}, err
	}
	return cloneClaim(claim), nil
}

func (s *Service) buildClaim(ctx context.Context, claimID string, token string, selectedItemIDs []string) (Claim, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Claim{}, fmt.Errorf("%w: token is required", ErrInvalidClaim)
	}
	if len(selectedItemIDs) == 0 {
		return Claim{}, fmt.Errorf("%w: at least one item must be selected", ErrInvalidClaim)
	}

	stored, err := s.ResolveToken(ctx, token)
	if err != nil {
		return Claim{}, err
	}

	itemsByID := make(map[string]PublicOrderItem, len(stored.Items))
	for _, item := range stored.Items {
		itemsByID[item.ItemID] = item
	}

	selected := make([]string, 0, len(selectedItemIDs))
	seen := map[string]bool{}
	totals := ingredient.MacroValues{}
	for _, rawItemID := range selectedItemIDs {
		itemID := strings.TrimSpace(rawItemID)
		if itemID == "" {
			return Claim{}, fmt.Errorf("%w: item id is required", ErrInvalidClaim)
		}
		if seen[itemID] {
			return Claim{}, fmt.Errorf("%w: item %s selected multiple times", ErrInvalidClaim, itemID)
		}
		item, ok := itemsByID[itemID]
		if !ok {
			return Claim{}, fmt.Errorf("%w: item %s not found for token", ErrInvalidClaim, itemID)
		}
		seen[itemID] = true
		selected = append(selected, itemID)
		totals = totals.Add(item.Macros)
	}

	now := s.now().UTC()
	return Claim{
		ID:              claimID,
		Token:           stored.Token,
		LocationID:      stored.LocationID,
		SelectedItemIDs: selected,
		Totals:          totals,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func expandClosedOrderItems(order contracts.ClosedOrder) ([]PublicOrderItem, error) {
	items := make([]PublicOrderItem, 0, len(order.Lines))
	seen := map[string]bool{}
	for lineIndex, line := range order.Lines {
		lineID := strings.TrimSpace(line.LineID)
		if lineID == "" {
			return nil, fmt.Errorf("%w: closed order line id is required", ErrInvalidTokenData)
		}
		if line.Quantity <= 0 {
			return nil, fmt.Errorf("%w: line %s has invalid quantity %d", ErrInvalidTokenData, lineID, line.Quantity)
		}
		perItemMacros := line.ResolvedMacros.Scale(1 / float64(line.Quantity))
		for i := 1; i <= line.Quantity; i++ {
			itemID := fmt.Sprintf("item-%d", lineIndex+1)
			if line.Quantity > 1 {
				itemID = fmt.Sprintf("%s-%d", itemID, i)
			}
			if seen[itemID] {
				return nil, fmt.Errorf("%w: duplicate token item id %s", ErrInvalidTokenData, itemID)
			}
			seen[itemID] = true
			items = append(items, PublicOrderItem{
				ItemID: itemID,
				LineID: lineID,
				Name:   line.Name,
				Macros: perItemMacros,
			})
		}
	}
	return items, nil
}

func (s *Service) newOpaqueID(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := io.ReadFull(s.rand, raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func cloneToken(token ReceiptToken) ReceiptToken {
	out := token
	out.Items = append([]PublicOrderItem(nil), token.Items...)
	return out
}

func cloneClaim(claim Claim) Claim {
	out := claim
	out.SelectedItemIDs = append([]string(nil), claim.SelectedItemIDs...)
	return out
}

func tokenExpired(now time.Time, expiresAt time.Time) bool {
	return !now.Before(expiresAt)
}
