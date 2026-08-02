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
var ErrInvalidReceiptData = errors.New("invalid receipt data")

type Repository interface {
	RecordDepletion(context.Context, contracts.ClosedOrder) ([]Movement, error)
	RecordReceipt(context.Context, contracts.PurchaseReceipt) ([]Movement, error)
	ListMovements(context.Context, string) ([]Movement, error)
	OnHand(context.Context, string) ([]OnHandItem, error)
	ListOrganizationMovements(context.Context, string, string) ([]Movement, error)
	OrganizationOnHand(context.Context, string, string) ([]OnHandItem, error)
}

type Movement struct {
	ID           string
	LocationID   string
	SourceType   string
	SourceID     string
	OrderID      string
	IngredientID string
	Quantity     float64
	Unit         ingredient.Unit
	OccurredAt   time.Time
}

type OnHandItem struct {
	LocationID     string
	IngredientID   string
	IngredientName string
	BaseUnit       ingredient.Unit
	OnHandQuantity float64
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

func (s *Service) OnHand(ctx context.Context, locationID string) ([]OnHandItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("inventory repository is required")
	}
	locationID = strings.TrimSpace(locationID)
	if locationID == "" {
		return nil, fmt.Errorf("location id is required")
	}
	return s.repo.OnHand(ctx, locationID)
}

func (s *Service) OrganizationMovements(ctx context.Context, organizationID string, locationID string) ([]Movement, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("inventory repository is required")
	}
	organizationID = strings.TrimSpace(organizationID)
	locationID = strings.TrimSpace(locationID)
	if organizationID == "" {
		return nil, fmt.Errorf("organization id is required")
	}
	return s.repo.ListOrganizationMovements(ctx, organizationID, locationID)
}

func (s *Service) OrganizationOnHand(ctx context.Context, organizationID string, locationID string) ([]OnHandItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("inventory repository is required")
	}
	organizationID = strings.TrimSpace(organizationID)
	locationID = strings.TrimSpace(locationID)
	if organizationID == "" {
		return nil, fmt.Errorf("organization id is required")
	}
	return s.repo.OrganizationOnHand(ctx, organizationID, locationID)
}

func (s *Service) RecordReceipt(ctx context.Context, receipt contracts.PurchaseReceipt) ([]Movement, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("inventory repository is required")
	}
	receipt.LocationID = strings.TrimSpace(receipt.LocationID)
	receipt.ReceiptID = strings.TrimSpace(receipt.ReceiptID)
	if receipt.LocationID == "" {
		return nil, fmt.Errorf("%w: receipt location id is required", ErrInvalidReceiptData)
	}
	if receipt.ReceiptID == "" {
		return nil, fmt.Errorf("%w: receipt id is required", ErrInvalidReceiptData)
	}
	if receipt.OccurredAt.IsZero() {
		return nil, fmt.Errorf("%w: receipt %s must have an occurred timestamp", ErrInvalidReceiptData, receipt.ReceiptID)
	}
	if len(receipt.Lines) == 0 {
		return nil, fmt.Errorf("%w: receipt %s must have at least one line", ErrInvalidReceiptData, receipt.ReceiptID)
	}
	if receipt.SourceType == "" {
		receipt.SourceType = contracts.PurchaseReceiptSourceType
	}
	if receipt.SourceID == "" {
		receipt.SourceID = receipt.ReceiptID
	}
	for _, line := range receipt.Lines {
		if strings.TrimSpace(line.IngredientID) == "" {
			return nil, fmt.Errorf("%w: receipt %s has a line with missing ingredient id", ErrInvalidReceiptData, receipt.ReceiptID)
		}
		if line.QuantityBaseUnits <= 0 {
			return nil, fmt.Errorf("%w: receipt %s has a line with non-positive quantity", ErrInvalidReceiptData, receipt.ReceiptID)
		}
		if line.Unit == "" {
			return nil, fmt.Errorf("%w: receipt %s has a line with missing unit", ErrInvalidReceiptData, receipt.ReceiptID)
		}
	}
	return s.repo.RecordReceipt(ctx, receipt)
}
