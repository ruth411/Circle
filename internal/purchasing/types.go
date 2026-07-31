package purchasing

import (
	"time"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
)

type UnitCost int64

func NewUnitCost(value float64) UnitCost {
	return UnitCost(ingredient.MustCostPerBaseUnit(value))
}

func MustUnitCost(value float64) UnitCost {
	return NewUnitCost(value)
}

func (c UnitCost) Float64() float64 {
	return ingredient.CostPerBaseUnit(c).Float64()
}

type VendorStatus string

const (
	VendorStatusActive   VendorStatus = "active"
	VendorStatusInactive VendorStatus = "inactive"
)

type VendorItemStatus string

const (
	VendorItemStatusActive   VendorItemStatus = "active"
	VendorItemStatusInactive VendorItemStatus = "inactive"
)

type PurchaseOrderStatus string

const (
	PurchaseOrderStatusDraft             PurchaseOrderStatus = "draft"
	PurchaseOrderStatusSubmitted         PurchaseOrderStatus = "submitted"
	PurchaseOrderStatusPartiallyReceived PurchaseOrderStatus = "partially_received"
	PurchaseOrderStatusReceived          PurchaseOrderStatus = "received"
	PurchaseOrderStatusCancelled         PurchaseOrderStatus = "cancelled"
)

type Vendor struct {
	ID          string
	LocationID  string
	Name        string
	ExternalRef string
	Status      VendorStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type VendorItem struct {
	ID                     string
	LocationID             string
	VendorID               string
	IngredientID           string
	VendorSKU              string
	Name                   string
	PurchaseUnit           string
	PackQuantity           float64
	IngredientBaseQuantity float64
	LastUnitCost           UnitCost
	Currency               string
	Status                 VendorItemStatus
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type PurchaseOrder struct {
	ID         string
	LocationID string
	PONumber   string
	VendorID   string
	Status     PurchaseOrderStatus
	OrderedAt  *time.Time
	Notes      string
	Lines      []PurchaseOrderLine
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type PurchaseOrderLine struct {
	ID               string
	LocationID       string
	PurchaseOrderID  string
	VendorItemID     string
	OrderedQuantity  float64
	OrderedUnitCost  UnitCost
	Currency         string
	ReceivedQuantity float64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Receipt struct {
	ID              string
	LocationID      string
	PurchaseOrderID string
	ReceivedAt      time.Time
	ReceivedBy      string
	Notes           string
	Lines           []ReceiptLine
	CreatedAt       time.Time
}

type ReceiptLine struct {
	ID                  string
	LocationID          string
	ReceiptID           string
	PurchaseOrderLineID string
	IngredientID        string
	ReceivedQuantity    float64
	ReceivedUnitCost    UnitCost
	Currency            string
	InventoryQuantity   float64
	InventoryUnit       ingredient.Unit
	CreatedAt           time.Time
}

type VendorInput struct {
	ID          string
	LocationID  string
	Name        string
	ExternalRef string
	Status      VendorStatus
}

type VendorItemInput struct {
	ID                     string
	LocationID             string
	VendorID               string
	IngredientID           string
	VendorSKU              string
	Name                   string
	PurchaseUnit           string
	PackQuantity           float64
	IngredientBaseQuantity float64
	LastUnitCost           float64
	Currency               string
	Status                 VendorItemStatus
}

type PurchaseOrderInput struct {
	ID         string
	LocationID string
	PONumber   string
	VendorID   string
	Notes      string
}

type PurchaseOrderLineInput struct {
	ID              string
	LocationID      string
	PurchaseOrderID string
	VendorItemID    string
	OrderedQuantity float64
	OrderedUnitCost float64
	Currency        string
}

type ReceiptInput struct {
	ID              string
	LocationID      string
	PurchaseOrderID string
	ReceivedAt      time.Time
	ReceivedBy      string
	Notes           string
	Lines           []ReceiptLineInput
}

type ReceiptLineInput struct {
	ID                  string
	PurchaseOrderLineID string
	ReceivedQuantity    float64
	ReceivedUnitCost    float64
	Currency            string
}

type ReceivedCostUpdate struct {
	LocationID      string
	IngredientID    string
	CostPerBaseUnit ingredient.CostPerBaseUnit
	ReceivedAt      time.Time
}

type PlannedReceipt struct {
	Receipt          Receipt
	InventoryReceipt contracts.PurchaseReceipt
	CostUpdates      []ReceivedCostUpdate
}
