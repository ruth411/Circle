package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/purchasing"
	"github.com/ruth411/circle/internal/tenancy"
)

type fakePurchasingRepository struct {
	createVendorFn     func(context.Context, purchasing.Vendor) (purchasing.Vendor, error)
	updateVendorFn     func(context.Context, purchasing.Vendor) (purchasing.Vendor, error)
	getVendorFn        func(context.Context, string, string) (purchasing.Vendor, error)
	listVendorsFn      func(context.Context, string, string) ([]purchasing.Vendor, error)
	createVendorItemFn func(context.Context, purchasing.VendorItem) (purchasing.VendorItem, error)
	updateVendorItemFn func(context.Context, purchasing.VendorItem) (purchasing.VendorItem, error)
	getVendorItemFn    func(context.Context, string, string) (purchasing.VendorItem, error)
	listVendorItemsFn  func(context.Context, string, string) ([]purchasing.VendorItem, error)
	createPOFn         func(context.Context, purchasing.PurchaseOrder) (purchasing.PurchaseOrder, error)
	getPOFn            func(context.Context, string, string) (purchasing.PurchaseOrder, error)
	listPOsFn          func(context.Context, string) ([]purchasing.PurchaseOrder, error)
	addPOLineFn        func(context.Context, string, string, purchasing.PurchaseOrderLine) (purchasing.PurchaseOrderLine, error)
	updatePOLineFn     func(context.Context, string, string, purchasing.PurchaseOrderLine) (purchasing.PurchaseOrderLine, error)
	removePOLineFn     func(context.Context, string, string, string) error
	submitPOFn         func(context.Context, string, string) (purchasing.PurchaseOrder, error)
	cancelPOFn         func(context.Context, string, string) (purchasing.PurchaseOrder, error)
	receiveFn          func(context.Context, purchasing.PlannedReceipt) (purchasing.Receipt, error)
	listReceiptsFn     func(context.Context, string) ([]purchasing.Receipt, error)
	getReceiptFn       func(context.Context, string, string) (purchasing.Receipt, error)
}

func (f fakePurchasingRepository) CreateVendor(ctx context.Context, vendor purchasing.Vendor) (purchasing.Vendor, error) {
	return f.createVendorFn(ctx, vendor)
}
func (f fakePurchasingRepository) UpdateVendor(ctx context.Context, vendor purchasing.Vendor) (purchasing.Vendor, error) {
	return f.updateVendorFn(ctx, vendor)
}
func (f fakePurchasingRepository) GetVendor(ctx context.Context, locationID string, vendorID string) (purchasing.Vendor, error) {
	return f.getVendorFn(ctx, locationID, vendorID)
}
func (f fakePurchasingRepository) ListVendors(ctx context.Context, locationID string, search string) ([]purchasing.Vendor, error) {
	return f.listVendorsFn(ctx, locationID, search)
}
func (f fakePurchasingRepository) CreateVendorItem(ctx context.Context, item purchasing.VendorItem) (purchasing.VendorItem, error) {
	return f.createVendorItemFn(ctx, item)
}
func (f fakePurchasingRepository) UpdateVendorItem(ctx context.Context, item purchasing.VendorItem) (purchasing.VendorItem, error) {
	return f.updateVendorItemFn(ctx, item)
}
func (f fakePurchasingRepository) GetVendorItem(ctx context.Context, locationID string, itemID string) (purchasing.VendorItem, error) {
	return f.getVendorItemFn(ctx, locationID, itemID)
}
func (f fakePurchasingRepository) ListVendorItems(ctx context.Context, locationID string, search string) ([]purchasing.VendorItem, error) {
	return f.listVendorItemsFn(ctx, locationID, search)
}
func (f fakePurchasingRepository) CreatePurchaseOrder(ctx context.Context, po purchasing.PurchaseOrder) (purchasing.PurchaseOrder, error) {
	return f.createPOFn(ctx, po)
}
func (f fakePurchasingRepository) GetPurchaseOrder(ctx context.Context, locationID string, poID string) (purchasing.PurchaseOrder, error) {
	return f.getPOFn(ctx, locationID, poID)
}
func (f fakePurchasingRepository) ListPurchaseOrders(ctx context.Context, locationID string) ([]purchasing.PurchaseOrder, error) {
	return f.listPOsFn(ctx, locationID)
}
func (f fakePurchasingRepository) AddPurchaseOrderLine(ctx context.Context, locationID string, poID string, line purchasing.PurchaseOrderLine) (purchasing.PurchaseOrderLine, error) {
	return f.addPOLineFn(ctx, locationID, poID, line)
}
func (f fakePurchasingRepository) UpdatePurchaseOrderLine(ctx context.Context, locationID string, poID string, line purchasing.PurchaseOrderLine) (purchasing.PurchaseOrderLine, error) {
	return f.updatePOLineFn(ctx, locationID, poID, line)
}
func (f fakePurchasingRepository) RemovePurchaseOrderLine(ctx context.Context, locationID string, poID string, lineID string) error {
	return f.removePOLineFn(ctx, locationID, poID, lineID)
}
func (f fakePurchasingRepository) SubmitPurchaseOrder(ctx context.Context, locationID string, poID string) (purchasing.PurchaseOrder, error) {
	return f.submitPOFn(ctx, locationID, poID)
}
func (f fakePurchasingRepository) CancelPurchaseOrder(ctx context.Context, locationID string, poID string) (purchasing.PurchaseOrder, error) {
	return f.cancelPOFn(ctx, locationID, poID)
}
func (f fakePurchasingRepository) Receive(ctx context.Context, planned purchasing.PlannedReceipt) (purchasing.Receipt, error) {
	return f.receiveFn(ctx, planned)
}
func (f fakePurchasingRepository) ListReceipts(ctx context.Context, locationID string) ([]purchasing.Receipt, error) {
	return f.listReceiptsFn(ctx, locationID)
}
func (f fakePurchasingRepository) GetReceipt(ctx context.Context, locationID string, receiptID string) (purchasing.Receipt, error) {
	return f.getReceiptFn(ctx, locationID, receiptID)
}

type fakePurchasingIngredientLookup struct {
	getFn func(context.Context, string, string) (ingredient.Ingredient, error)
}

func (f fakePurchasingIngredientLookup) Get(ctx context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
	return f.getFn(ctx, locationID, ingredientID)
}

func TestVendorCreateRouteCreatesVendor(t *testing.T) {
	service := purchasing.NewService(fakePurchasingRepository{
		createVendorFn: func(_ context.Context, vendor purchasing.Vendor) (purchasing.Vendor, error) {
			return vendor, nil
		},
	}, fakePurchasingIngredientLookup{})

	identityService := seedSessionService(t, "loc-1")
	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		PurchasingService:    service,
		SessionValidator:     identityService,
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPost, "/vendors", bytes.NewBufferString(`{"id":"ven-1","name":"Main Supplier"}`))
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", recorder.Code)
	}
	var payload struct {
		Vendor vendorResponse `json:"vendor"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Vendor.ID != "ven-1" {
		t.Fatalf("vendor id = %q, want ven-1", payload.Vendor.ID)
	}
}

func TestReceiptCreateRouteRoundsUnitCost(t *testing.T) {
	service := purchasing.NewService(fakePurchasingRepository{
		getPOFn: func(_ context.Context, locationID string, poID string) (purchasing.PurchaseOrder, error) {
			return purchasing.PurchaseOrder{
				ID:         poID,
				LocationID: locationID,
				VendorID:   "ven-1",
				Status:     purchasing.PurchaseOrderStatusSubmitted,
				Lines: []purchasing.PurchaseOrderLine{
					{
						ID:               "line-1",
						LocationID:       locationID,
						PurchaseOrderID:  poID,
						VendorItemID:     "item-1",
						OrderedQuantity:  5,
						OrderedUnitCost:  purchasing.MustUnitCost(8),
						Currency:         "USD",
						ReceivedQuantity: 0,
					},
				},
			}, nil
		},
		getVendorItemFn: func(_ context.Context, locationID string, itemID string) (purchasing.VendorItem, error) {
			return purchasing.VendorItem{
				ID:                     itemID,
				LocationID:             locationID,
				VendorID:               "ven-1",
				IngredientID:           "ing-1",
				Name:                   "Chicken Case",
				PurchaseUnit:           "case",
				PackQuantity:           2,
				IngredientBaseQuantity: 1000,
				LastUnitCost:           purchasing.MustUnitCost(9),
				Currency:               "USD",
				Status:                 purchasing.VendorItemStatusActive,
			}, nil
		},
		receiveFn: func(_ context.Context, planned purchasing.PlannedReceipt) (purchasing.Receipt, error) {
			return planned.Receipt, nil
		},
	}, fakePurchasingIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{ID: ingredientID, LocationID: locationID, BaseUnit: ingredient.UnitGram}, nil
		},
	})

	identityService := seedSessionService(t, "loc-1")
	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		PurchasingService:    service,
		SessionValidator:     identityService,
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	body := bytes.NewBufferString(`{
		"id":"rec-1",
		"purchase_order_id":"po-1",
		"received_at":"2026-07-30T12:00:00Z",
		"lines":[{"id":"rline-1","purchase_order_line_id":"line-1","received_quantity":3,"received_unit_cost":9.99999,"currency":"USD"}]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/receipts", body)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", recorder.Code)
	}
	var payload struct {
		Receipt receiptResponse `json:"receipt"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if got, want := payload.Receipt.Lines[0].ReceivedUnitCost, 10.0; got != want {
		t.Fatalf("received unit cost = %v, want %v", got, want)
	}
}

func TestPurchaseOrderCancelRouteCancelsPurchaseOrder(t *testing.T) {
	service := purchasing.NewService(fakePurchasingRepository{
		getPOFn: func(_ context.Context, locationID string, poID string) (purchasing.PurchaseOrder, error) {
			return purchasing.PurchaseOrder{
				ID:         poID,
				LocationID: locationID,
				Status:     purchasing.PurchaseOrderStatusSubmitted,
			}, nil
		},
		cancelPOFn: func(_ context.Context, locationID string, poID string) (purchasing.PurchaseOrder, error) {
			return purchasing.PurchaseOrder{
				ID:         poID,
				LocationID: locationID,
				Status:     purchasing.PurchaseOrderStatusCancelled,
			}, nil
		},
	}, fakePurchasingIngredientLookup{})

	identityService := seedSessionService(t, "loc-1")
	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		PurchasingService:    service,
		SessionValidator:     identityService,
		OrganizationResolver: tenancy.StaticOrganizationResolver{"loc-1": "org-1"},
	})

	req := httptest.NewRequest(http.MethodPost, "/purchase-orders/po-1/cancel", nil)
	req.Header.Set("X-Location-Id", "loc-1")
	req.Header.Set(sessionIDHeader, "session-1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var payload struct {
		PurchaseOrder purchaseOrderResponse `json:"purchase_order"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.PurchaseOrder.Status != purchasing.PurchaseOrderStatusCancelled {
		t.Fatalf("status = %q, want cancelled", payload.PurchaseOrder.Status)
	}
}
