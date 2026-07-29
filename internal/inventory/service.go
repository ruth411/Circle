package inventory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
)

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
		return nil, fmt.Errorf("closed order location id is required")
	}
	if strings.TrimSpace(order.OrderID) == "" {
		return nil, fmt.Errorf("closed order id is required")
	}
	if order.ClosedAt.IsZero() {
		return nil, fmt.Errorf("order %s must have a closed timestamp", order.OrderID)
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
