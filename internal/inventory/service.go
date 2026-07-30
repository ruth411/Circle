package inventory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
)

var ErrInvalidClosedOrderData = errors.New("invalid closed order data")

type Repository interface {
	RecordDepletion(context.Context, contracts.ClosedOrder) ([]Movement, error)
	ListMovements(context.Context, string) ([]Movement, error)
}

type Movement struct {
	ID           string
	LocationID   string
	OrderID      string
	IngredientID string
	Quantity     float64
	Unit         ingredient.Unit
	OccurredAt   time.Time
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) RecordDepletion(ctx context.Context, order contracts.ClosedOrder) ([]Movement, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("inventory repository is required")
	}
	order.LocationID = strings.TrimSpace(order.LocationID)
	if order.LocationID == "" {
		return nil, fmt.Errorf("%w: closed order location id is required", ErrInvalidClosedOrderData)
	}
	if strings.TrimSpace(order.OrderID) == "" {
		return nil, fmt.Errorf("%w: closed order id is required", ErrInvalidClosedOrderData)
	}
	if order.ClosedAt.IsZero() {
		return nil, fmt.Errorf("%w: order %s must have a closed timestamp", ErrInvalidClosedOrderData, order.OrderID)
	}
	return s.repo.RecordDepletion(ctx, order)
}

func (s *Service) Movements(ctx context.Context, locationID string) ([]Movement, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("inventory repository is required")
	}
	locationID = strings.TrimSpace(locationID)
	if locationID == "" {
		return nil, fmt.Errorf("location id is required")
	}
	return s.repo.ListMovements(ctx, locationID)
}
