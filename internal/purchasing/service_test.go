package purchasing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
)

type fakeRepository struct {
	createVendorFn     func(context.Context, Vendor) (Vendor, error)
	updateVendorFn     func(context.Context, Vendor) (Vendor, error)
	getVendorFn        func(context.Context, string, string) (Vendor, error)
	listVendorsFn      func(context.Context, string, string) ([]Vendor, error)
	createVendorItemFn func(context.Context, VendorItem) (VendorItem, error)
	updateVendorItemFn func(context.Context, VendorItem) (VendorItem, error)
	getVendorItemFn    func(context.Context, string, string) (VendorItem, error)
	listVendorItemsFn  func(context.Context, string, string) ([]VendorItem, error)
	createPOFn         func(context.Context, PurchaseOrder) (PurchaseOrder, error)
	getPOFn            func(context.Context, string, string) (PurchaseOrder, error)
	listPOsFn          func(context.Context, string) ([]PurchaseOrder, error)
	addPOLineFn        func(context.Context, string, string, PurchaseOrderLine) (PurchaseOrderLine, error)
	updatePOLineFn     func(context.Context, string, string, PurchaseOrderLine) (PurchaseOrderLine, error)
	removePOLineFn     func(context.Context, string, string, string) error
	submitPOFn         func(context.Context, string, string) (PurchaseOrder, error)
	receiveFn          func(context.Context, PlannedReceipt) (Receipt, error)
	listReceiptsFn     func(context.Context, string) ([]Receipt, error)
	getReceiptFn       func(context.Context, string, string) (Receipt, error)
}

func (f fakeRepository) CreateVendor(ctx context.Context, vendor Vendor) (Vendor, error) {
	return f.createVendorFn(ctx, vendor)
}

func (f fakeRepository) UpdateVendor(ctx context.Context, vendor Vendor) (Vendor, error) {
	return f.updateVendorFn(ctx, vendor)
}

func (f fakeRepository) GetVendor(ctx context.Context, locationID string, vendorID string) (Vendor, error) {
	return f.getVendorFn(ctx, locationID, vendorID)
}

func (f fakeRepository) ListVendors(ctx context.Context, locationID string, search string) ([]Vendor, error) {
	return f.listVendorsFn(ctx, locationID, search)
}

func (f fakeRepository) CreateVendorItem(ctx context.Context, item VendorItem) (VendorItem, error) {
	return f.createVendorItemFn(ctx, item)
}

func (f fakeRepository) UpdateVendorItem(ctx context.Context, item VendorItem) (VendorItem, error) {
	return f.updateVendorItemFn(ctx, item)
}

func (f fakeRepository) GetVendorItem(ctx context.Context, locationID string, itemID string) (VendorItem, error) {
	return f.getVendorItemFn(ctx, locationID, itemID)
}

func (f fakeRepository) ListVendorItems(ctx context.Context, locationID string, search string) ([]VendorItem, error) {
	return f.listVendorItemsFn(ctx, locationID, search)
}

func (f fakeRepository) CreatePurchaseOrder(ctx context.Context, po PurchaseOrder) (PurchaseOrder, error) {
	return f.createPOFn(ctx, po)
}

func (f fakeRepository) GetPurchaseOrder(ctx context.Context, locationID string, poID string) (PurchaseOrder, error) {
	return f.getPOFn(ctx, locationID, poID)
}

func (f fakeRepository) ListPurchaseOrders(ctx context.Context, locationID string) ([]PurchaseOrder, error) {
	return f.listPOsFn(ctx, locationID)
}

func (f fakeRepository) AddPurchaseOrderLine(ctx context.Context, locationID string, poID string, line PurchaseOrderLine) (PurchaseOrderLine, error) {
	return f.addPOLineFn(ctx, locationID, poID, line)
}

func (f fakeRepository) UpdatePurchaseOrderLine(ctx context.Context, locationID string, poID string, line PurchaseOrderLine) (PurchaseOrderLine, error) {
	return f.updatePOLineFn(ctx, locationID, poID, line)
}

func (f fakeRepository) RemovePurchaseOrderLine(ctx context.Context, locationID string, poID string, lineID string) error {
	return f.removePOLineFn(ctx, locationID, poID, lineID)
}

func (f fakeRepository) SubmitPurchaseOrder(ctx context.Context, locationID string, poID string) (PurchaseOrder, error) {
	return f.submitPOFn(ctx, locationID, poID)
}

func (f fakeRepository) Receive(ctx context.Context, planned PlannedReceipt) (Receipt, error) {
	return f.receiveFn(ctx, planned)
}

func (f fakeRepository) ListReceipts(ctx context.Context, locationID string) ([]Receipt, error) {
	return f.listReceiptsFn(ctx, locationID)
}

func (f fakeRepository) GetReceipt(ctx context.Context, locationID string, receiptID string) (Receipt, error) {
	return f.getReceiptFn(ctx, locationID, receiptID)
}

type fakeIngredientLookup struct {
	getFn func(context.Context, string, string) (ingredient.Ingredient, error)
}

func (f fakeIngredientLookup) Get(ctx context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
	return f.getFn(ctx, locationID, ingredientID)
}

func TestCreateVendorTrimsAndDefaultsActive(t *testing.T) {
	service := NewService(fakeRepository{
		createVendorFn: func(_ context.Context, vendor Vendor) (Vendor, error) {
			return vendor, nil
		},
	}, fakeIngredientLookup{})

	vendor, err := service.CreateVendor(context.Background(), VendorInput{
		ID:         " ven-1 ",
		LocationID: " loc-1 ",
		Name:       " Main Supplier ",
	})
	if err != nil {
		t.Fatalf("CreateVendor returned error: %v", err)
	}
	if vendor.ID != "ven-1" {
		t.Fatalf("vendor id = %q, want ven-1", vendor.ID)
	}
	if vendor.LocationID != "loc-1" {
		t.Fatalf("vendor location = %q, want loc-1", vendor.LocationID)
	}
	if vendor.Name != "Main Supplier" {
		t.Fatalf("vendor name = %q, want Main Supplier", vendor.Name)
	}
	if vendor.Status != VendorStatusActive {
		t.Fatalf("vendor status = %q, want active", vendor.Status)
	}
}

func TestCreateVendorItemRejectsCrossLocationIngredient(t *testing.T) {
	service := NewService(fakeRepository{
		getVendorFn: func(_ context.Context, locationID string, vendorID string) (Vendor, error) {
			return Vendor{ID: vendorID, LocationID: locationID, Status: VendorStatusActive}, nil
		},
		createVendorItemFn: func(_ context.Context, item VendorItem) (VendorItem, error) {
			return item, nil
		},
	}, fakeIngredientLookup{
		getFn: func(_ context.Context, _ string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{ID: ingredientID, LocationID: "loc-2", BaseUnit: ingredient.UnitGram}, nil
		},
	})

	_, err := service.CreateVendorItem(context.Background(), VendorItemInput{
		ID:                     "item-1",
		LocationID:             "loc-1",
		VendorID:               "ven-1",
		IngredientID:           "ing-1",
		Name:                   "Chicken Case",
		PurchaseUnit:           "case",
		PackQuantity:           1,
		IngredientBaseQuantity: 40000,
		LastUnitCost:           89.1234,
		Currency:               "USD",
	})
	if !errors.Is(err, ErrInvalidPurchase) {
		t.Fatalf("err = %v, want ErrInvalidPurchase", err)
	}
}

func TestSubmitPurchaseOrderRequiresAtLeastOneLine(t *testing.T) {
	service := NewService(fakeRepository{
		getPOFn: func(_ context.Context, locationID string, poID string) (PurchaseOrder, error) {
			return PurchaseOrder{
				ID:         poID,
				LocationID: locationID,
				VendorID:   "ven-1",
				Status:     PurchaseOrderStatusDraft,
			}, nil
		},
		submitPOFn: func(_ context.Context, _ string, _ string) (PurchaseOrder, error) {
			t.Fatal("SubmitPurchaseOrder should not be called")
			return PurchaseOrder{}, nil
		},
	}, fakeIngredientLookup{})

	_, err := service.SubmitPurchaseOrder(context.Background(), "loc-1", "po-1")
	if !errors.Is(err, ErrInvalidPurchase) {
		t.Fatalf("err = %v, want ErrInvalidPurchase", err)
	}
}

func TestAddPurchaseOrderLineRejectsVendorItemFromAnotherVendor(t *testing.T) {
	service := NewService(fakeRepository{
		getPOFn: func(_ context.Context, locationID string, poID string) (PurchaseOrder, error) {
			return PurchaseOrder{
				ID:         poID,
				LocationID: locationID,
				VendorID:   "ven-a",
				Status:     PurchaseOrderStatusDraft,
			}, nil
		},
		getVendorItemFn: func(_ context.Context, locationID string, itemID string) (VendorItem, error) {
			return VendorItem{
				ID:         itemID,
				LocationID: locationID,
				VendorID:   "ven-b",
				Status:     VendorItemStatusActive,
			}, nil
		},
		addPOLineFn: func(_ context.Context, _, _ string, _ PurchaseOrderLine) (PurchaseOrderLine, error) {
			t.Fatal("AddPurchaseOrderLine should not be called")
			return PurchaseOrderLine{}, nil
		},
	}, fakeIngredientLookup{})

	_, err := service.AddPurchaseOrderLine(context.Background(), PurchaseOrderLineInput{
		ID:              "line-1",
		LocationID:      "loc-1",
		PurchaseOrderID: "po-1",
		VendorItemID:    "item-1",
		OrderedQuantity: 1,
		OrderedUnitCost: 10,
		Currency:        "USD",
	})
	if !errors.Is(err, ErrInvalidPurchase) {
		t.Fatalf("err = %v, want ErrInvalidPurchase", err)
	}
}

func TestUpdatePurchaseOrderLineRejectsVendorItemFromAnotherVendor(t *testing.T) {
	service := NewService(fakeRepository{
		getPOFn: func(_ context.Context, locationID string, poID string) (PurchaseOrder, error) {
			return PurchaseOrder{
				ID:         poID,
				LocationID: locationID,
				VendorID:   "ven-a",
				Status:     PurchaseOrderStatusDraft,
			}, nil
		},
		getVendorItemFn: func(_ context.Context, locationID string, itemID string) (VendorItem, error) {
			return VendorItem{
				ID:         itemID,
				LocationID: locationID,
				VendorID:   "ven-b",
				Status:     VendorItemStatusActive,
			}, nil
		},
		updatePOLineFn: func(_ context.Context, _, _ string, _ PurchaseOrderLine) (PurchaseOrderLine, error) {
			t.Fatal("UpdatePurchaseOrderLine should not be called")
			return PurchaseOrderLine{}, nil
		},
	}, fakeIngredientLookup{})

	_, err := service.UpdatePurchaseOrderLine(context.Background(), PurchaseOrderLineInput{
		ID:              "line-1",
		LocationID:      "loc-1",
		PurchaseOrderID: "po-1",
		VendorItemID:    "item-1",
		OrderedQuantity: 1,
		OrderedUnitCost: 10,
		Currency:        "USD",
	})
	if !errors.Is(err, ErrInvalidPurchase) {
		t.Fatalf("err = %v, want ErrInvalidPurchase", err)
	}
}

func TestAddPurchaseOrderLineRejectsCurrencyMismatchWithVendorItem(t *testing.T) {
	service := NewService(fakeRepository{
		getPOFn: func(_ context.Context, locationID string, poID string) (PurchaseOrder, error) {
			return PurchaseOrder{
				ID:         poID,
				LocationID: locationID,
				VendorID:   "ven-1",
				Status:     PurchaseOrderStatusDraft,
			}, nil
		},
		getVendorItemFn: func(_ context.Context, locationID string, itemID string) (VendorItem, error) {
			return VendorItem{
				ID:         itemID,
				LocationID: locationID,
				VendorID:   "ven-1",
				Currency:   "USD",
				Status:     VendorItemStatusActive,
			}, nil
		},
		addPOLineFn: func(_ context.Context, _, _ string, _ PurchaseOrderLine) (PurchaseOrderLine, error) {
			t.Fatal("AddPurchaseOrderLine should not be called")
			return PurchaseOrderLine{}, nil
		},
	}, fakeIngredientLookup{})

	_, err := service.AddPurchaseOrderLine(context.Background(), PurchaseOrderLineInput{
		ID:              "line-1",
		LocationID:      "loc-1",
		PurchaseOrderID: "po-1",
		VendorItemID:    "item-1",
		OrderedQuantity: 1,
		OrderedUnitCost: 10,
		Currency:        "EUR",
	})
	if !errors.Is(err, ErrInvalidPurchase) {
		t.Fatalf("err = %v, want ErrInvalidPurchase", err)
	}
}

func TestUpdatePurchaseOrderLineRejectsCurrencyMismatchWithVendorItem(t *testing.T) {
	service := NewService(fakeRepository{
		getPOFn: func(_ context.Context, locationID string, poID string) (PurchaseOrder, error) {
			return PurchaseOrder{
				ID:         poID,
				LocationID: locationID,
				VendorID:   "ven-1",
				Status:     PurchaseOrderStatusDraft,
			}, nil
		},
		getVendorItemFn: func(_ context.Context, locationID string, itemID string) (VendorItem, error) {
			return VendorItem{
				ID:         itemID,
				LocationID: locationID,
				VendorID:   "ven-1",
				Currency:   "USD",
				Status:     VendorItemStatusActive,
			}, nil
		},
		updatePOLineFn: func(_ context.Context, _, _ string, _ PurchaseOrderLine) (PurchaseOrderLine, error) {
			t.Fatal("UpdatePurchaseOrderLine should not be called")
			return PurchaseOrderLine{}, nil
		},
	}, fakeIngredientLookup{})

	_, err := service.UpdatePurchaseOrderLine(context.Background(), PurchaseOrderLineInput{
		ID:              "line-1",
		LocationID:      "loc-1",
		PurchaseOrderID: "po-1",
		VendorItemID:    "item-1",
		OrderedQuantity: 1,
		OrderedUnitCost: 10,
		Currency:        "EUR",
	})
	if !errors.Is(err, ErrInvalidPurchase) {
		t.Fatalf("err = %v, want ErrInvalidPurchase", err)
	}
}

func TestReceiveBuildsInventoryAndCostUpdates(t *testing.T) {
	var got PlannedReceipt
	service := NewService(fakeRepository{
		getPOFn: func(_ context.Context, locationID string, poID string) (PurchaseOrder, error) {
			return PurchaseOrder{
				ID:         poID,
				LocationID: locationID,
				VendorID:   "ven-1",
				Status:     PurchaseOrderStatusSubmitted,
				Lines: []PurchaseOrderLine{
					{
						ID:               "line-1",
						LocationID:       locationID,
						PurchaseOrderID:  poID,
						VendorItemID:     "item-1",
						OrderedQuantity:  5,
						OrderedUnitCost:  MustUnitCost(8),
						Currency:         "USD",
						ReceivedQuantity: 1,
					},
				},
			}, nil
		},
		getVendorItemFn: func(_ context.Context, locationID string, itemID string) (VendorItem, error) {
			return VendorItem{
				ID:                     itemID,
				LocationID:             locationID,
				VendorID:               "ven-1",
				IngredientID:           "ing-1",
				Name:                   "Chicken Case",
				PurchaseUnit:           "case",
				PackQuantity:           2,
				IngredientBaseQuantity: 1000,
				LastUnitCost:           MustUnitCost(9),
				Currency:               "USD",
				Status:                 VendorItemStatusActive,
			}, nil
		},
		receiveFn: func(_ context.Context, planned PlannedReceipt) (Receipt, error) {
			got = planned
			return planned.Receipt, nil
		},
	}, fakeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{
				ID:                     ingredientID,
				LocationID:             locationID,
				BaseUnit:               ingredient.UnitGram,
				CurrentCostPerBaseUnit: ingredient.MustCostPerBaseUnit(0.0100),
			}, nil
		},
	})

	receivedAt := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	receipt, err := service.Receive(context.Background(), ReceiptInput{
		ID:              "rec-1",
		LocationID:      "loc-1",
		PurchaseOrderID: "po-1",
		ReceivedAt:      receivedAt,
		ReceivedBy:      "staff-1",
		Lines: []ReceiptLineInput{
			{
				ID:                  "rline-1",
				PurchaseOrderLineID: "line-1",
				ReceivedQuantity:    3,
				ReceivedUnitCost:    9.99999,
				Currency:            "USD",
			},
		},
	})
	if err != nil {
		t.Fatalf("Receive returned error: %v", err)
	}
	if receipt.ID != "rec-1" {
		t.Fatalf("receipt id = %q, want rec-1", receipt.ID)
	}
	if len(got.InventoryReceipt.Lines) != 1 {
		t.Fatalf("inventory receipt line count = %d, want 1", len(got.InventoryReceipt.Lines))
	}
	if qty := got.InventoryReceipt.Lines[0].QuantityBaseUnits; qty != 1500 {
		t.Fatalf("inventory quantity = %v, want 1500", qty)
	}
	if unit := got.InventoryReceipt.Lines[0].Unit; unit != ingredient.UnitGram {
		t.Fatalf("inventory unit = %q, want g", unit)
	}
	if len(got.CostUpdates) != 1 {
		t.Fatalf("cost update count = %d, want 1", len(got.CostUpdates))
	}
	if got.CostUpdates[0].CostPerBaseUnit.Float64() != 0.02 {
		t.Fatalf("cost per base unit = %v, want 0.02", got.CostUpdates[0].CostPerBaseUnit.Float64())
	}
	if got.Receipt.Lines[0].ReceivedUnitCost.Float64() != 10.0 {
		t.Fatalf("received unit cost = %v, want 10.0", got.Receipt.Lines[0].ReceivedUnitCost.Float64())
	}
}

func TestReceiveRejectsOverReceipt(t *testing.T) {
	service := NewService(fakeRepository{
		getPOFn: func(_ context.Context, locationID string, poID string) (PurchaseOrder, error) {
			return PurchaseOrder{
				ID:         poID,
				LocationID: locationID,
				VendorID:   "ven-1",
				Status:     PurchaseOrderStatusSubmitted,
				Lines: []PurchaseOrderLine{
					{
						ID:               "line-1",
						LocationID:       locationID,
						PurchaseOrderID:  poID,
						VendorItemID:     "item-1",
						OrderedQuantity:  2,
						ReceivedQuantity: 1,
						OrderedUnitCost:  MustUnitCost(8),
						Currency:         "USD",
					},
				},
			}, nil
		},
		getVendorItemFn: func(_ context.Context, locationID string, itemID string) (VendorItem, error) {
			return VendorItem{
				ID:                     itemID,
				LocationID:             locationID,
				VendorID:               "ven-1",
				IngredientID:           "ing-1",
				Name:                   "Chicken Case",
				PurchaseUnit:           "case",
				PackQuantity:           1,
				IngredientBaseQuantity: 1000,
				LastUnitCost:           MustUnitCost(9),
				Currency:               "USD",
				Status:                 VendorItemStatusActive,
			}, nil
		},
		receiveFn: func(_ context.Context, planned PlannedReceipt) (Receipt, error) {
			t.Fatal("Receive should not be called")
			return Receipt{}, nil
		},
	}, fakeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{ID: ingredientID, LocationID: locationID, BaseUnit: ingredient.UnitGram}, nil
		},
	})

	_, err := service.Receive(context.Background(), ReceiptInput{
		ID:              "rec-1",
		LocationID:      "loc-1",
		PurchaseOrderID: "po-1",
		ReceivedAt:      time.Now().UTC(),
		Lines: []ReceiptLineInput{
			{
				ID:                  "rline-1",
				PurchaseOrderLineID: "line-1",
				ReceivedQuantity:    2,
				ReceivedUnitCost:    10,
				Currency:            "USD",
			},
		},
	})
	if !errors.Is(err, ErrInvalidPurchase) {
		t.Fatalf("err = %v, want ErrInvalidPurchase", err)
	}
}

func TestReceiveRejectsReceiptCurrencyMismatch(t *testing.T) {
	service := NewService(fakeRepository{
		getPOFn: func(_ context.Context, locationID string, poID string) (PurchaseOrder, error) {
			return PurchaseOrder{
				ID:         poID,
				LocationID: locationID,
				VendorID:   "ven-1",
				Status:     PurchaseOrderStatusSubmitted,
				Lines: []PurchaseOrderLine{
					{
						ID:               "line-1",
						LocationID:       locationID,
						PurchaseOrderID:  poID,
						VendorItemID:     "item-1",
						OrderedQuantity:  2,
						ReceivedQuantity: 0,
						OrderedUnitCost:  MustUnitCost(8),
						Currency:         "USD",
					},
				},
			}, nil
		},
		getVendorItemFn: func(_ context.Context, locationID string, itemID string) (VendorItem, error) {
			return VendorItem{
				ID:                     itemID,
				LocationID:             locationID,
				VendorID:               "ven-1",
				IngredientID:           "ing-1",
				Name:                   "Chicken Case",
				PurchaseUnit:           "case",
				PackQuantity:           1,
				IngredientBaseQuantity: 1000,
				LastUnitCost:           MustUnitCost(9),
				Currency:               "USD",
				Status:                 VendorItemStatusActive,
			}, nil
		},
		receiveFn: func(_ context.Context, _ PlannedReceipt) (Receipt, error) {
			t.Fatal("Receive should not be called")
			return Receipt{}, nil
		},
	}, fakeIngredientLookup{
		getFn: func(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
			return ingredient.Ingredient{ID: ingredientID, LocationID: locationID, BaseUnit: ingredient.UnitGram}, nil
		},
	})

	_, err := service.Receive(context.Background(), ReceiptInput{
		ID:              "rec-1",
		LocationID:      "loc-1",
		PurchaseOrderID: "po-1",
		ReceivedAt:      time.Now().UTC(),
		Lines: []ReceiptLineInput{
			{
				ID:                  "rline-1",
				PurchaseOrderLineID: "line-1",
				ReceivedQuantity:    1,
				ReceivedUnitCost:    10,
				Currency:            "EUR",
			},
		},
	})
	if !errors.Is(err, ErrInvalidPurchase) {
		t.Fatalf("err = %v, want ErrInvalidPurchase", err)
	}
}

func TestNormalizePurchaseInputsRejectNegativeCosts(t *testing.T) {
	cases := []struct {
		name string
		fn   func() error
	}{
		{
			name: "vendor item",
			fn: func() error {
				_, err := normalizeVendorItem(VendorItemInput{
					ID:                     "item-1",
					LocationID:             "loc-1",
					VendorID:               "ven-1",
					IngredientID:           "ing-1",
					Name:                   "Chicken Case",
					PurchaseUnit:           "case",
					PackQuantity:           1,
					IngredientBaseQuantity: 1000,
					LastUnitCost:           -1,
					Currency:               "USD",
				})
				return err
			},
		},
		{
			name: "po line",
			fn: func() error {
				_, err := normalizePurchaseOrderLine(PurchaseOrderLineInput{
					ID:              "line-1",
					LocationID:      "loc-1",
					PurchaseOrderID: "po-1",
					VendorItemID:    "item-1",
					OrderedQuantity: 1,
					OrderedUnitCost: -1,
					Currency:        "USD",
				})
				return err
			},
		},
		{
			name: "receipt line",
			fn: func() error {
				_, err := normalizeReceipt(ReceiptInput{
					ID:              "rec-1",
					LocationID:      "loc-1",
					PurchaseOrderID: "po-1",
					ReceivedAt:      time.Now().UTC(),
					Lines: []ReceiptLineInput{
						{
							ID:                  "rline-1",
							PurchaseOrderLineID: "line-1",
							ReceivedQuantity:    1,
							ReceivedUnitCost:    -1,
							Currency:            "USD",
						},
					},
				})
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(); !errors.Is(err, ErrInvalidPurchase) {
				t.Fatalf("err = %v, want ErrInvalidPurchase", err)
			}
		})
	}
}

var _ contracts.PurchaseReceipt
