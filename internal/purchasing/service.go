package purchasing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
)

var (
	ErrVendorNotFound        = errors.New("vendor not found")
	ErrVendorItemNotFound    = errors.New("vendor item not found")
	ErrPurchaseOrderNotFound = errors.New("purchase order not found")
	ErrReceiptNotFound       = errors.New("receipt not found")
	ErrInvalidPurchase       = errors.New("invalid purchase")
)

type Repository interface {
	CreateVendor(context.Context, Vendor) (Vendor, error)
	UpdateVendor(context.Context, Vendor) (Vendor, error)
	GetVendor(context.Context, string, string) (Vendor, error)
	ListVendors(context.Context, string, string) ([]Vendor, error)
	CreateVendorItem(context.Context, VendorItem) (VendorItem, error)
	UpdateVendorItem(context.Context, VendorItem) (VendorItem, error)
	GetVendorItem(context.Context, string, string) (VendorItem, error)
	ListVendorItems(context.Context, string, string) ([]VendorItem, error)
	CreatePurchaseOrder(context.Context, PurchaseOrder) (PurchaseOrder, error)
	GetPurchaseOrder(context.Context, string, string) (PurchaseOrder, error)
	ListPurchaseOrders(context.Context, string) ([]PurchaseOrder, error)
	AddPurchaseOrderLine(context.Context, string, string, PurchaseOrderLine) (PurchaseOrderLine, error)
	UpdatePurchaseOrderLine(context.Context, string, string, PurchaseOrderLine) (PurchaseOrderLine, error)
	RemovePurchaseOrderLine(context.Context, string, string, string) error
	SubmitPurchaseOrder(context.Context, string, string) (PurchaseOrder, error)
	CancelPurchaseOrder(context.Context, string, string) (PurchaseOrder, error)
	Receive(context.Context, PlannedReceipt) (Receipt, error)
	ListReceipts(context.Context, string) ([]Receipt, error)
	GetReceipt(context.Context, string, string) (Receipt, error)
}

type IngredientLookup interface {
	Get(context.Context, string, string) (ingredient.Ingredient, error)
}

type Service struct {
	repo        Repository
	ingredients IngredientLookup
}

func NewService(repo Repository, ingredients IngredientLookup) *Service {
	return &Service{repo: repo, ingredients: ingredients}
}

func (s *Service) CreateVendor(ctx context.Context, input VendorInput) (Vendor, error) {
	if s.repo == nil {
		return Vendor{}, fmt.Errorf("purchasing repository is required")
	}
	vendor, err := normalizeVendor(input)
	if err != nil {
		return Vendor{}, err
	}
	return s.repo.CreateVendor(ctx, vendor)
}

func (s *Service) UpdateVendor(ctx context.Context, input VendorInput) (Vendor, error) {
	if s.repo == nil {
		return Vendor{}, fmt.Errorf("purchasing repository is required")
	}
	vendor, err := normalizeVendor(input)
	if err != nil {
		return Vendor{}, err
	}
	return s.repo.UpdateVendor(ctx, vendor)
}

func (s *Service) ListVendors(ctx context.Context, locationID string, search string) ([]Vendor, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("purchasing repository is required")
	}
	locationID = strings.TrimSpace(locationID)
	if locationID == "" {
		return nil, fmt.Errorf("%w: location id is required", ErrInvalidPurchase)
	}
	return s.repo.ListVendors(ctx, locationID, strings.TrimSpace(search))
}

func (s *Service) GetVendor(ctx context.Context, locationID string, vendorID string) (Vendor, error) {
	if s.repo == nil {
		return Vendor{}, fmt.Errorf("purchasing repository is required")
	}
	locationID = strings.TrimSpace(locationID)
	vendorID = strings.TrimSpace(vendorID)
	if locationID == "" || vendorID == "" {
		return Vendor{}, fmt.Errorf("%w: location id and vendor id are required", ErrInvalidPurchase)
	}
	return s.repo.GetVendor(ctx, locationID, vendorID)
}

func (s *Service) CreateVendorItem(ctx context.Context, input VendorItemInput) (VendorItem, error) {
	if s.repo == nil {
		return VendorItem{}, fmt.Errorf("purchasing repository is required")
	}
	if s.ingredients == nil {
		return VendorItem{}, fmt.Errorf("ingredient lookup is required")
	}
	item, err := normalizeVendorItem(input)
	if err != nil {
		return VendorItem{}, err
	}
	vendor, err := s.repo.GetVendor(ctx, item.LocationID, item.VendorID)
	if err != nil {
		return VendorItem{}, err
	}
	if vendor.Status != VendorStatusActive {
		return VendorItem{}, fmt.Errorf("%w: vendor %s is inactive", ErrInvalidPurchase, vendor.ID)
	}
	ing, err := s.ingredients.Get(ctx, item.LocationID, item.IngredientID)
	if err != nil {
		return VendorItem{}, err
	}
	if ing.LocationID != item.LocationID {
		return VendorItem{}, fmt.Errorf("%w: ingredient %s does not belong to location %s", ErrInvalidPurchase, ing.ID, item.LocationID)
	}
	return s.repo.CreateVendorItem(ctx, item)
}

func (s *Service) UpdateVendorItem(ctx context.Context, input VendorItemInput) (VendorItem, error) {
	if s.repo == nil {
		return VendorItem{}, fmt.Errorf("purchasing repository is required")
	}
	if s.ingredients == nil {
		return VendorItem{}, fmt.Errorf("ingredient lookup is required")
	}
	item, err := normalizeVendorItem(input)
	if err != nil {
		return VendorItem{}, err
	}
	vendor, err := s.repo.GetVendor(ctx, item.LocationID, item.VendorID)
	if err != nil {
		return VendorItem{}, err
	}
	if vendor.Status != VendorStatusActive {
		return VendorItem{}, fmt.Errorf("%w: vendor %s is inactive", ErrInvalidPurchase, vendor.ID)
	}
	ing, err := s.ingredients.Get(ctx, item.LocationID, item.IngredientID)
	if err != nil {
		return VendorItem{}, err
	}
	if ing.LocationID != item.LocationID {
		return VendorItem{}, fmt.Errorf("%w: ingredient %s does not belong to location %s", ErrInvalidPurchase, ing.ID, item.LocationID)
	}
	return s.repo.UpdateVendorItem(ctx, item)
}

func (s *Service) ListVendorItems(ctx context.Context, locationID string, search string) ([]VendorItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("purchasing repository is required")
	}
	locationID = strings.TrimSpace(locationID)
	if locationID == "" {
		return nil, fmt.Errorf("%w: location id is required", ErrInvalidPurchase)
	}
	return s.repo.ListVendorItems(ctx, locationID, strings.TrimSpace(search))
}

func (s *Service) GetVendorItem(ctx context.Context, locationID string, itemID string) (VendorItem, error) {
	if s.repo == nil {
		return VendorItem{}, fmt.Errorf("purchasing repository is required")
	}
	locationID = strings.TrimSpace(locationID)
	itemID = strings.TrimSpace(itemID)
	if locationID == "" || itemID == "" {
		return VendorItem{}, fmt.Errorf("%w: location id and vendor item id are required", ErrInvalidPurchase)
	}
	return s.repo.GetVendorItem(ctx, locationID, itemID)
}

func (s *Service) CreatePurchaseOrder(ctx context.Context, input PurchaseOrderInput) (PurchaseOrder, error) {
	if s.repo == nil {
		return PurchaseOrder{}, fmt.Errorf("purchasing repository is required")
	}
	po, err := normalizePurchaseOrder(input)
	if err != nil {
		return PurchaseOrder{}, err
	}
	vendor, err := s.repo.GetVendor(ctx, po.LocationID, po.VendorID)
	if err != nil {
		return PurchaseOrder{}, err
	}
	if vendor.Status != VendorStatusActive {
		return PurchaseOrder{}, fmt.Errorf("%w: vendor %s is inactive", ErrInvalidPurchase, vendor.ID)
	}
	return s.repo.CreatePurchaseOrder(ctx, po)
}

func (s *Service) GetPurchaseOrder(ctx context.Context, locationID string, poID string) (PurchaseOrder, error) {
	if s.repo == nil {
		return PurchaseOrder{}, fmt.Errorf("purchasing repository is required")
	}
	locationID = strings.TrimSpace(locationID)
	poID = strings.TrimSpace(poID)
	if locationID == "" || poID == "" {
		return PurchaseOrder{}, fmt.Errorf("%w: location id and purchase order id are required", ErrInvalidPurchase)
	}
	return s.repo.GetPurchaseOrder(ctx, locationID, poID)
}

func (s *Service) ListPurchaseOrders(ctx context.Context, locationID string) ([]PurchaseOrder, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("purchasing repository is required")
	}
	locationID = strings.TrimSpace(locationID)
	if locationID == "" {
		return nil, fmt.Errorf("%w: location id is required", ErrInvalidPurchase)
	}
	return s.repo.ListPurchaseOrders(ctx, locationID)
}

func (s *Service) AddPurchaseOrderLine(ctx context.Context, input PurchaseOrderLineInput) (PurchaseOrderLine, error) {
	if s.repo == nil {
		return PurchaseOrderLine{}, fmt.Errorf("purchasing repository is required")
	}
	line, err := normalizePurchaseOrderLine(input)
	if err != nil {
		return PurchaseOrderLine{}, err
	}
	po, err := s.repo.GetPurchaseOrder(ctx, line.LocationID, line.PurchaseOrderID)
	if err != nil {
		return PurchaseOrderLine{}, err
	}
	if po.Status != PurchaseOrderStatusDraft {
		return PurchaseOrderLine{}, fmt.Errorf("%w: purchase order %s is not editable", ErrInvalidPurchase, po.ID)
	}
	item, err := s.repo.GetVendorItem(ctx, line.LocationID, line.VendorItemID)
	if err != nil {
		return PurchaseOrderLine{}, err
	}
	if item.Status != VendorItemStatusActive {
		return PurchaseOrderLine{}, fmt.Errorf("%w: vendor item %s is inactive", ErrInvalidPurchase, item.ID)
	}
	if item.VendorID != po.VendorID {
		return PurchaseOrderLine{}, fmt.Errorf("%w: vendor item %s belongs to vendor %s, not purchase order vendor %s", ErrInvalidPurchase, item.ID, item.VendorID, po.VendorID)
	}
	if line.Currency != item.Currency {
		return PurchaseOrderLine{}, fmt.Errorf("%w: purchase order line currency %s does not match vendor item currency %s", ErrInvalidPurchase, line.Currency, item.Currency)
	}
	return s.repo.AddPurchaseOrderLine(ctx, line.LocationID, line.PurchaseOrderID, line)
}

func (s *Service) UpdatePurchaseOrderLine(ctx context.Context, input PurchaseOrderLineInput) (PurchaseOrderLine, error) {
	if s.repo == nil {
		return PurchaseOrderLine{}, fmt.Errorf("purchasing repository is required")
	}
	line, err := normalizePurchaseOrderLine(input)
	if err != nil {
		return PurchaseOrderLine{}, err
	}
	po, err := s.repo.GetPurchaseOrder(ctx, line.LocationID, line.PurchaseOrderID)
	if err != nil {
		return PurchaseOrderLine{}, err
	}
	if po.Status != PurchaseOrderStatusDraft {
		return PurchaseOrderLine{}, fmt.Errorf("%w: purchase order %s is not editable", ErrInvalidPurchase, po.ID)
	}
	item, err := s.repo.GetVendorItem(ctx, line.LocationID, line.VendorItemID)
	if err != nil {
		return PurchaseOrderLine{}, err
	}
	if item.Status != VendorItemStatusActive {
		return PurchaseOrderLine{}, fmt.Errorf("%w: vendor item %s is inactive", ErrInvalidPurchase, item.ID)
	}
	if item.VendorID != po.VendorID {
		return PurchaseOrderLine{}, fmt.Errorf("%w: vendor item %s belongs to vendor %s, not purchase order vendor %s", ErrInvalidPurchase, item.ID, item.VendorID, po.VendorID)
	}
	if line.Currency != item.Currency {
		return PurchaseOrderLine{}, fmt.Errorf("%w: purchase order line currency %s does not match vendor item currency %s", ErrInvalidPurchase, line.Currency, item.Currency)
	}
	return s.repo.UpdatePurchaseOrderLine(ctx, line.LocationID, line.PurchaseOrderID, line)
}

func (s *Service) RemovePurchaseOrderLine(ctx context.Context, locationID string, poID string, lineID string) error {
	if s.repo == nil {
		return fmt.Errorf("purchasing repository is required")
	}
	locationID = strings.TrimSpace(locationID)
	poID = strings.TrimSpace(poID)
	lineID = strings.TrimSpace(lineID)
	if locationID == "" || poID == "" || lineID == "" {
		return fmt.Errorf("%w: location id, purchase order id, and line id are required", ErrInvalidPurchase)
	}
	po, err := s.repo.GetPurchaseOrder(ctx, locationID, poID)
	if err != nil {
		return err
	}
	if po.Status != PurchaseOrderStatusDraft {
		return fmt.Errorf("%w: purchase order %s is not editable", ErrInvalidPurchase, po.ID)
	}
	return s.repo.RemovePurchaseOrderLine(ctx, locationID, poID, lineID)
}

func (s *Service) SubmitPurchaseOrder(ctx context.Context, locationID string, poID string) (PurchaseOrder, error) {
	if s.repo == nil {
		return PurchaseOrder{}, fmt.Errorf("purchasing repository is required")
	}
	locationID = strings.TrimSpace(locationID)
	poID = strings.TrimSpace(poID)
	if locationID == "" || poID == "" {
		return PurchaseOrder{}, fmt.Errorf("%w: location id and purchase order id are required", ErrInvalidPurchase)
	}
	po, err := s.repo.GetPurchaseOrder(ctx, locationID, poID)
	if err != nil {
		return PurchaseOrder{}, err
	}
	if po.Status != PurchaseOrderStatusDraft {
		return PurchaseOrder{}, fmt.Errorf("%w: purchase order %s is not draft", ErrInvalidPurchase, po.ID)
	}
	if len(po.Lines) == 0 {
		return PurchaseOrder{}, fmt.Errorf("%w: purchase order %s must have at least one line", ErrInvalidPurchase, po.ID)
	}
	return s.repo.SubmitPurchaseOrder(ctx, locationID, poID)
}

func (s *Service) CancelPurchaseOrder(ctx context.Context, locationID string, poID string) (PurchaseOrder, error) {
	if s.repo == nil {
		return PurchaseOrder{}, fmt.Errorf("purchasing repository is required")
	}
	locationID = strings.TrimSpace(locationID)
	poID = strings.TrimSpace(poID)
	if locationID == "" || poID == "" {
		return PurchaseOrder{}, fmt.Errorf("%w: location id and purchase order id are required", ErrInvalidPurchase)
	}
	po, err := s.repo.GetPurchaseOrder(ctx, locationID, poID)
	if err != nil {
		return PurchaseOrder{}, err
	}
	if po.Status != PurchaseOrderStatusDraft && po.Status != PurchaseOrderStatusSubmitted {
		return PurchaseOrder{}, fmt.Errorf("%w: purchase order %s cannot be cancelled from status %s", ErrInvalidPurchase, po.ID, po.Status)
	}
	return s.repo.CancelPurchaseOrder(ctx, locationID, poID)
}

func (s *Service) Receive(ctx context.Context, input ReceiptInput) (Receipt, error) {
	if s.repo == nil {
		return Receipt{}, fmt.Errorf("purchasing repository is required")
	}
	if s.ingredients == nil {
		return Receipt{}, fmt.Errorf("ingredient lookup is required")
	}
	receipt, err := normalizeReceipt(input)
	if err != nil {
		return Receipt{}, err
	}
	po, err := s.repo.GetPurchaseOrder(ctx, receipt.LocationID, receipt.PurchaseOrderID)
	if err != nil {
		return Receipt{}, err
	}
	if po.Status != PurchaseOrderStatusSubmitted && po.Status != PurchaseOrderStatusPartiallyReceived {
		return Receipt{}, fmt.Errorf("%w: purchase order %s is not receivable", ErrInvalidPurchase, po.ID)
	}

	lineByID := make(map[string]PurchaseOrderLine, len(po.Lines))
	for _, line := range po.Lines {
		lineByID[line.ID] = line
	}

	planned := PlannedReceipt{
		Receipt: receipt,
		InventoryReceipt: contracts.PurchaseReceipt{
			ReceiptID:  receipt.ID,
			LocationID: receipt.LocationID,
			OccurredAt: receipt.ReceivedAt.UTC(),
			SourceType: contracts.PurchaseReceiptSourceType,
			SourceID:   receipt.ID,
		},
	}

	for i := range planned.Receipt.Lines {
		line := &planned.Receipt.Lines[i]
		poLine, ok := lineByID[line.PurchaseOrderLineID]
		if !ok {
			return Receipt{}, fmt.Errorf("%w: receipt line %s references unknown purchase order line %s", ErrInvalidPurchase, line.ID, line.PurchaseOrderLineID)
		}
		remaining := poLine.OrderedQuantity - poLine.ReceivedQuantity
		if line.ReceivedQuantity > remaining {
			return Receipt{}, fmt.Errorf("%w: receipt line %s exceeds remaining quantity for purchase order line %s", ErrInvalidPurchase, line.ID, line.PurchaseOrderLineID)
		}
		item, err := s.repo.GetVendorItem(ctx, receipt.LocationID, poLine.VendorItemID)
		if err != nil {
			return Receipt{}, err
		}
		ing, err := s.ingredients.Get(ctx, receipt.LocationID, item.IngredientID)
		if err != nil {
			return Receipt{}, err
		}
		if ing.LocationID != receipt.LocationID {
			return Receipt{}, fmt.Errorf("%w: ingredient %s does not belong to location %s", ErrInvalidPurchase, ing.ID, receipt.LocationID)
		}
		if line.Currency != poLine.Currency || line.Currency != item.Currency {
			return Receipt{}, fmt.Errorf("%w: receipt line %s currency %s does not match purchase order currency", ErrInvalidPurchase, line.ID, line.Currency)
		}
		perPurchaseUnit := item.IngredientBaseQuantity / item.PackQuantity
		inventoryQuantity := line.ReceivedQuantity * perPurchaseUnit
		line.IngredientID = item.IngredientID
		line.InventoryQuantity = inventoryQuantity
		line.InventoryUnit = ing.BaseUnit

		planned.InventoryReceipt.Lines = append(planned.InventoryReceipt.Lines, contracts.PurchaseReceiptLine{
			IngredientID:      item.IngredientID,
			QuantityBaseUnits: inventoryQuantity,
			Unit:              ing.BaseUnit,
		})

		planned.CostUpdates = append(planned.CostUpdates, ReceivedCostUpdate{
			LocationID:      receipt.LocationID,
			IngredientID:    item.IngredientID,
			CostPerBaseUnit: ingredient.NewCostPerBaseUnit(line.ReceivedUnitCost.Float64() / perPurchaseUnit),
			ReceivedAt:      receipt.ReceivedAt.UTC(),
		})
	}

	return s.repo.Receive(ctx, planned)
}

func (s *Service) ListReceipts(ctx context.Context, locationID string) ([]Receipt, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("purchasing repository is required")
	}
	locationID = strings.TrimSpace(locationID)
	if locationID == "" {
		return nil, fmt.Errorf("%w: location id is required", ErrInvalidPurchase)
	}
	return s.repo.ListReceipts(ctx, locationID)
}

func (s *Service) GetReceipt(ctx context.Context, locationID string, receiptID string) (Receipt, error) {
	if s.repo == nil {
		return Receipt{}, fmt.Errorf("purchasing repository is required")
	}
	locationID = strings.TrimSpace(locationID)
	receiptID = strings.TrimSpace(receiptID)
	if locationID == "" || receiptID == "" {
		return Receipt{}, fmt.Errorf("%w: location id and receipt id are required", ErrInvalidPurchase)
	}
	return s.repo.GetReceipt(ctx, locationID, receiptID)
}

func normalizeVendor(input VendorInput) (Vendor, error) {
	vendor := Vendor{
		ID:          strings.TrimSpace(input.ID),
		LocationID:  strings.TrimSpace(input.LocationID),
		Name:        strings.TrimSpace(input.Name),
		ExternalRef: strings.TrimSpace(input.ExternalRef),
		Status:      input.Status,
	}
	if vendor.Status == "" {
		vendor.Status = VendorStatusActive
	}
	if vendor.ID == "" || vendor.LocationID == "" || vendor.Name == "" {
		return Vendor{}, fmt.Errorf("%w: vendor id, location id, and name are required", ErrInvalidPurchase)
	}
	if vendor.Status != VendorStatusActive && vendor.Status != VendorStatusInactive {
		return Vendor{}, fmt.Errorf("%w: vendor status is invalid", ErrInvalidPurchase)
	}
	return vendor, nil
}

func normalizeVendorItem(input VendorItemInput) (VendorItem, error) {
	item := VendorItem{
		ID:                     strings.TrimSpace(input.ID),
		LocationID:             strings.TrimSpace(input.LocationID),
		VendorID:               strings.TrimSpace(input.VendorID),
		IngredientID:           strings.TrimSpace(input.IngredientID),
		VendorSKU:              strings.TrimSpace(input.VendorSKU),
		Name:                   strings.TrimSpace(input.Name),
		PurchaseUnit:           strings.TrimSpace(input.PurchaseUnit),
		PackQuantity:           input.PackQuantity,
		IngredientBaseQuantity: input.IngredientBaseQuantity,
		LastUnitCost:           NewUnitCost(input.LastUnitCost),
		Currency:               strings.ToUpper(strings.TrimSpace(input.Currency)),
		Status:                 input.Status,
	}
	if item.Status == "" {
		item.Status = VendorItemStatusActive
	}
	if item.ID == "" || item.LocationID == "" || item.VendorID == "" || item.IngredientID == "" || item.Name == "" || item.PurchaseUnit == "" || item.Currency == "" {
		return VendorItem{}, fmt.Errorf("%w: vendor item id, location id, vendor id, ingredient id, name, purchase unit, and currency are required", ErrInvalidPurchase)
	}
	if item.PackQuantity <= 0 || item.IngredientBaseQuantity <= 0 {
		return VendorItem{}, fmt.Errorf("%w: pack quantity and ingredient base quantity must be positive", ErrInvalidPurchase)
	}
	if input.LastUnitCost < 0 {
		return VendorItem{}, fmt.Errorf("%w: last unit cost must be non-negative", ErrInvalidPurchase)
	}
	if item.Status != VendorItemStatusActive && item.Status != VendorItemStatusInactive {
		return VendorItem{}, fmt.Errorf("%w: vendor item status is invalid", ErrInvalidPurchase)
	}
	return item, nil
}

func normalizePurchaseOrder(input PurchaseOrderInput) (PurchaseOrder, error) {
	po := PurchaseOrder{
		ID:         strings.TrimSpace(input.ID),
		LocationID: strings.TrimSpace(input.LocationID),
		PONumber:   strings.TrimSpace(input.PONumber),
		VendorID:   strings.TrimSpace(input.VendorID),
		Notes:      strings.TrimSpace(input.Notes),
		Status:     PurchaseOrderStatusDraft,
	}
	if po.ID == "" || po.LocationID == "" || po.PONumber == "" || po.VendorID == "" {
		return PurchaseOrder{}, fmt.Errorf("%w: purchase order id, location id, po number, and vendor id are required", ErrInvalidPurchase)
	}
	return po, nil
}

func normalizePurchaseOrderLine(input PurchaseOrderLineInput) (PurchaseOrderLine, error) {
	line := PurchaseOrderLine{
		ID:              strings.TrimSpace(input.ID),
		LocationID:      strings.TrimSpace(input.LocationID),
		PurchaseOrderID: strings.TrimSpace(input.PurchaseOrderID),
		VendorItemID:    strings.TrimSpace(input.VendorItemID),
		OrderedQuantity: input.OrderedQuantity,
		OrderedUnitCost: NewUnitCost(input.OrderedUnitCost),
		Currency:        strings.ToUpper(strings.TrimSpace(input.Currency)),
	}
	if line.ID == "" || line.LocationID == "" || line.PurchaseOrderID == "" || line.VendorItemID == "" || line.Currency == "" {
		return PurchaseOrderLine{}, fmt.Errorf("%w: purchase order line id, location id, purchase order id, vendor item id, and currency are required", ErrInvalidPurchase)
	}
	if line.OrderedQuantity <= 0 {
		return PurchaseOrderLine{}, fmt.Errorf("%w: ordered quantity must be positive", ErrInvalidPurchase)
	}
	if input.OrderedUnitCost < 0 {
		return PurchaseOrderLine{}, fmt.Errorf("%w: ordered unit cost must be non-negative", ErrInvalidPurchase)
	}
	return line, nil
}

func normalizeReceipt(input ReceiptInput) (Receipt, error) {
	receipt := Receipt{
		ID:              strings.TrimSpace(input.ID),
		LocationID:      strings.TrimSpace(input.LocationID),
		PurchaseOrderID: strings.TrimSpace(input.PurchaseOrderID),
		ReceivedAt:      input.ReceivedAt.UTC(),
		ReceivedBy:      strings.TrimSpace(input.ReceivedBy),
		Notes:           strings.TrimSpace(input.Notes),
		Lines:           make([]ReceiptLine, len(input.Lines)),
	}
	if receipt.ID == "" || receipt.LocationID == "" || receipt.PurchaseOrderID == "" {
		return Receipt{}, fmt.Errorf("%w: receipt id, location id, and purchase order id are required", ErrInvalidPurchase)
	}
	if receipt.ReceivedAt.IsZero() {
		return Receipt{}, fmt.Errorf("%w: received at is required", ErrInvalidPurchase)
	}
	if len(input.Lines) == 0 {
		return Receipt{}, fmt.Errorf("%w: at least one receipt line is required", ErrInvalidPurchase)
	}
	for i, line := range input.Lines {
		receiptLine := ReceiptLine{
			ID:                  strings.TrimSpace(line.ID),
			LocationID:          receipt.LocationID,
			ReceiptID:           receipt.ID,
			PurchaseOrderLineID: strings.TrimSpace(line.PurchaseOrderLineID),
			ReceivedQuantity:    line.ReceivedQuantity,
			ReceivedUnitCost:    NewUnitCost(line.ReceivedUnitCost),
			Currency:            strings.ToUpper(strings.TrimSpace(line.Currency)),
		}
		if receiptLine.ID == "" || receiptLine.PurchaseOrderLineID == "" || receiptLine.Currency == "" {
			return Receipt{}, fmt.Errorf("%w: receipt line id, purchase order line id, and currency are required", ErrInvalidPurchase)
		}
		if receiptLine.ReceivedQuantity <= 0 {
			return Receipt{}, fmt.Errorf("%w: received quantity must be positive", ErrInvalidPurchase)
		}
		if line.ReceivedUnitCost < 0 {
			return Receipt{}, fmt.Errorf("%w: received unit cost must be non-negative", ErrInvalidPurchase)
		}
		receipt.Lines[i] = receiptLine
	}
	return receipt, nil
}
