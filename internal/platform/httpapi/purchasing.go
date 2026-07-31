package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/ruth411/circle/internal/purchasing"
	"github.com/ruth411/circle/internal/tenancy"
)

const maxPurchaseRequestBodyBytes int64 = 1 << 20

type purchasingDependencies struct {
	service              *purchasing.Service
	locationResolver     tenancy.Resolver
	organizationResolver tenancy.OrganizationResolver
	sessionValidator     SessionValidator
}

type vendorRequest struct {
	ID          string                  `json:"id,omitempty"`
	Name        string                  `json:"name"`
	ExternalRef string                  `json:"external_ref,omitempty"`
	Status      purchasing.VendorStatus `json:"status,omitempty"`
}

type vendorItemRequest struct {
	ID                     string                      `json:"id,omitempty"`
	VendorID               string                      `json:"vendor_id"`
	IngredientID           string                      `json:"ingredient_id"`
	VendorSKU              string                      `json:"vendor_sku,omitempty"`
	Name                   string                      `json:"name"`
	PurchaseUnit           string                      `json:"purchase_unit"`
	PackQuantity           float64                     `json:"pack_quantity"`
	IngredientBaseQuantity float64                     `json:"ingredient_base_quantity"`
	LastUnitCost           float64                     `json:"last_unit_cost"`
	Currency               string                      `json:"currency"`
	Status                 purchasing.VendorItemStatus `json:"status,omitempty"`
}

type purchaseOrderRequest struct {
	ID       string `json:"id,omitempty"`
	PONumber string `json:"po_number"`
	VendorID string `json:"vendor_id"`
	Notes    string `json:"notes,omitempty"`
}

type purchaseOrderLineRequest struct {
	ID              string  `json:"id,omitempty"`
	VendorItemID    string  `json:"vendor_item_id"`
	OrderedQuantity float64 `json:"ordered_quantity"`
	OrderedUnitCost float64 `json:"ordered_unit_cost"`
	Currency        string  `json:"currency"`
}

type receiptRequest struct {
	ID              string               `json:"id,omitempty"`
	PurchaseOrderID string               `json:"purchase_order_id"`
	ReceivedAt      time.Time            `json:"received_at"`
	ReceivedBy      string               `json:"received_by,omitempty"`
	Notes           string               `json:"notes,omitempty"`
	Lines           []receiptLineRequest `json:"lines"`
}

type receiptLineRequest struct {
	ID                  string  `json:"id,omitempty"`
	PurchaseOrderLineID string  `json:"purchase_order_line_id"`
	ReceivedQuantity    float64 `json:"received_quantity"`
	ReceivedUnitCost    float64 `json:"received_unit_cost"`
	Currency            string  `json:"currency"`
}

type vendorResponse struct {
	ID          string                  `json:"id"`
	LocationID  string                  `json:"location_id"`
	Name        string                  `json:"name"`
	ExternalRef string                  `json:"external_ref,omitempty"`
	Status      purchasing.VendorStatus `json:"status"`
}

type vendorItemResponse struct {
	ID                     string                      `json:"id"`
	LocationID             string                      `json:"location_id"`
	VendorID               string                      `json:"vendor_id"`
	IngredientID           string                      `json:"ingredient_id"`
	VendorSKU              string                      `json:"vendor_sku,omitempty"`
	Name                   string                      `json:"name"`
	PurchaseUnit           string                      `json:"purchase_unit"`
	PackQuantity           float64                     `json:"pack_quantity"`
	IngredientBaseQuantity float64                     `json:"ingredient_base_quantity"`
	LastUnitCost           float64                     `json:"last_unit_cost"`
	Currency               string                      `json:"currency"`
	Status                 purchasing.VendorItemStatus `json:"status"`
}

type purchaseOrderResponse struct {
	ID         string                         `json:"id"`
	LocationID string                         `json:"location_id"`
	PONumber   string                         `json:"po_number"`
	VendorID   string                         `json:"vendor_id"`
	Status     purchasing.PurchaseOrderStatus `json:"status"`
	OrderedAt  *time.Time                     `json:"ordered_at,omitempty"`
	Notes      string                         `json:"notes,omitempty"`
	Lines      []purchaseOrderLineResponse    `json:"lines"`
}

type purchaseOrderLineResponse struct {
	ID               string  `json:"id"`
	LocationID       string  `json:"location_id"`
	PurchaseOrderID  string  `json:"purchase_order_id"`
	VendorItemID     string  `json:"vendor_item_id"`
	OrderedQuantity  float64 `json:"ordered_quantity"`
	OrderedUnitCost  float64 `json:"ordered_unit_cost"`
	Currency         string  `json:"currency"`
	ReceivedQuantity float64 `json:"received_quantity"`
}

type receiptResponse struct {
	ID              string                `json:"id"`
	LocationID      string                `json:"location_id"`
	PurchaseOrderID string                `json:"purchase_order_id"`
	ReceivedAt      time.Time             `json:"received_at"`
	ReceivedBy      string                `json:"received_by,omitempty"`
	Notes           string                `json:"notes,omitempty"`
	Lines           []receiptLineResponse `json:"lines"`
}

type receiptLineResponse struct {
	ID                  string  `json:"id"`
	LocationID          string  `json:"location_id"`
	ReceiptID           string  `json:"receipt_id"`
	PurchaseOrderLineID string  `json:"purchase_order_line_id"`
	IngredientID        string  `json:"ingredient_id"`
	ReceivedQuantity    float64 `json:"received_quantity"`
	ReceivedUnitCost    float64 `json:"received_unit_cost"`
	Currency            string  `json:"currency"`
	InventoryQuantity   float64 `json:"inventory_quantity"`
	InventoryUnit       string  `json:"inventory_unit"`
}

func registerPurchasingRoutes(mux *http.ServeMux, deps purchasingDependencies) {
	if deps.service == nil || deps.sessionValidator == nil {
		return
	}

	resolver := deps.locationResolver
	if resolver == nil {
		resolver = tenancy.HeaderResolver{}
	}

	protected := func(next http.Handler) http.Handler {
		return WithResolvedLocation(resolver, RequireStaffSession(deps.sessionValidator, deps.organizationResolver, next))
	}

	mux.Handle("GET /vendors", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		vendors, err := deps.service.ListVendors(r.Context(), locationID, r.URL.Query().Get("q"))
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		payload := make([]vendorResponse, len(vendors))
		for i, vendor := range vendors {
			payload[i] = toVendorResponse(vendor)
		}
		WriteJSON(w, http.StatusOK, map[string]any{"vendors": payload, "request_id": RequestID(r.Context())})
	})))

	mux.Handle("POST /vendors", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload vendorRequest
		if err := decodePurchaseRequest(w, r, &payload); err != nil {
			writePurchaseDecodeError(w, r, err)
			return
		}
		locationID, _ := tenancy.LocationID(r.Context())
		vendor, err := deps.service.CreateVendor(r.Context(), purchasing.VendorInput{
			ID:          payload.ID,
			LocationID:  locationID,
			Name:        payload.Name,
			ExternalRef: payload.ExternalRef,
			Status:      payload.Status,
		})
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusCreated, map[string]any{"vendor": toVendorResponse(vendor), "request_id": RequestID(r.Context())})
	})))

	mux.Handle("GET /vendors/{id}", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		vendor, err := deps.service.GetVendor(r.Context(), locationID, r.PathValue("id"))
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"vendor": toVendorResponse(vendor), "request_id": RequestID(r.Context())})
	})))

	mux.Handle("PUT /vendors/{id}", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload vendorRequest
		if err := decodePurchaseRequest(w, r, &payload); err != nil {
			writePurchaseDecodeError(w, r, err)
			return
		}
		locationID, _ := tenancy.LocationID(r.Context())
		vendor, err := deps.service.UpdateVendor(r.Context(), purchasing.VendorInput{
			ID:          r.PathValue("id"),
			LocationID:  locationID,
			Name:        payload.Name,
			ExternalRef: payload.ExternalRef,
			Status:      payload.Status,
		})
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"vendor": toVendorResponse(vendor), "request_id": RequestID(r.Context())})
	})))

	mux.Handle("GET /vendor-items", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		items, err := deps.service.ListVendorItems(r.Context(), locationID, r.URL.Query().Get("q"))
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		payload := make([]vendorItemResponse, len(items))
		for i, item := range items {
			payload[i] = toVendorItemResponse(item)
		}
		WriteJSON(w, http.StatusOK, map[string]any{"vendor_items": payload, "request_id": RequestID(r.Context())})
	})))

	mux.Handle("POST /vendor-items", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload vendorItemRequest
		if err := decodePurchaseRequest(w, r, &payload); err != nil {
			writePurchaseDecodeError(w, r, err)
			return
		}
		locationID, _ := tenancy.LocationID(r.Context())
		item, err := deps.service.CreateVendorItem(r.Context(), purchasing.VendorItemInput{
			ID:                     payload.ID,
			LocationID:             locationID,
			VendorID:               payload.VendorID,
			IngredientID:           payload.IngredientID,
			VendorSKU:              payload.VendorSKU,
			Name:                   payload.Name,
			PurchaseUnit:           payload.PurchaseUnit,
			PackQuantity:           payload.PackQuantity,
			IngredientBaseQuantity: payload.IngredientBaseQuantity,
			LastUnitCost:           payload.LastUnitCost,
			Currency:               payload.Currency,
			Status:                 payload.Status,
		})
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusCreated, map[string]any{"vendor_item": toVendorItemResponse(item), "request_id": RequestID(r.Context())})
	})))

	mux.Handle("GET /vendor-items/{id}", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		item, err := deps.service.GetVendorItem(r.Context(), locationID, r.PathValue("id"))
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"vendor_item": toVendorItemResponse(item), "request_id": RequestID(r.Context())})
	})))

	mux.Handle("PUT /vendor-items/{id}", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload vendorItemRequest
		if err := decodePurchaseRequest(w, r, &payload); err != nil {
			writePurchaseDecodeError(w, r, err)
			return
		}
		locationID, _ := tenancy.LocationID(r.Context())
		item, err := deps.service.UpdateVendorItem(r.Context(), purchasing.VendorItemInput{
			ID:                     r.PathValue("id"),
			LocationID:             locationID,
			VendorID:               payload.VendorID,
			IngredientID:           payload.IngredientID,
			VendorSKU:              payload.VendorSKU,
			Name:                   payload.Name,
			PurchaseUnit:           payload.PurchaseUnit,
			PackQuantity:           payload.PackQuantity,
			IngredientBaseQuantity: payload.IngredientBaseQuantity,
			LastUnitCost:           payload.LastUnitCost,
			Currency:               payload.Currency,
			Status:                 payload.Status,
		})
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"vendor_item": toVendorItemResponse(item), "request_id": RequestID(r.Context())})
	})))

	mux.Handle("GET /purchase-orders", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		orders, err := deps.service.ListPurchaseOrders(r.Context(), locationID)
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		payload := make([]purchaseOrderResponse, len(orders))
		for i, order := range orders {
			payload[i] = toPurchaseOrderResponse(order)
		}
		WriteJSON(w, http.StatusOK, map[string]any{"purchase_orders": payload, "request_id": RequestID(r.Context())})
	})))

	mux.Handle("POST /purchase-orders", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload purchaseOrderRequest
		if err := decodePurchaseRequest(w, r, &payload); err != nil {
			writePurchaseDecodeError(w, r, err)
			return
		}
		locationID, _ := tenancy.LocationID(r.Context())
		order, err := deps.service.CreatePurchaseOrder(r.Context(), purchasing.PurchaseOrderInput{
			ID:         payload.ID,
			LocationID: locationID,
			PONumber:   payload.PONumber,
			VendorID:   payload.VendorID,
			Notes:      payload.Notes,
		})
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusCreated, map[string]any{"purchase_order": toPurchaseOrderResponse(order), "request_id": RequestID(r.Context())})
	})))

	mux.Handle("GET /purchase-orders/{id}", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		order, err := deps.service.GetPurchaseOrder(r.Context(), locationID, r.PathValue("id"))
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"purchase_order": toPurchaseOrderResponse(order), "request_id": RequestID(r.Context())})
	})))

	mux.Handle("POST /purchase-orders/{id}/lines", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload purchaseOrderLineRequest
		if err := decodePurchaseRequest(w, r, &payload); err != nil {
			writePurchaseDecodeError(w, r, err)
			return
		}
		locationID, _ := tenancy.LocationID(r.Context())
		line, err := deps.service.AddPurchaseOrderLine(r.Context(), purchasing.PurchaseOrderLineInput{
			ID:              payload.ID,
			LocationID:      locationID,
			PurchaseOrderID: r.PathValue("id"),
			VendorItemID:    payload.VendorItemID,
			OrderedQuantity: payload.OrderedQuantity,
			OrderedUnitCost: payload.OrderedUnitCost,
			Currency:        payload.Currency,
		})
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusCreated, map[string]any{"line": toPurchaseOrderLineResponse(line), "request_id": RequestID(r.Context())})
	})))

	mux.Handle("PUT /purchase-orders/{id}/lines/{line_id}", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload purchaseOrderLineRequest
		if err := decodePurchaseRequest(w, r, &payload); err != nil {
			writePurchaseDecodeError(w, r, err)
			return
		}
		locationID, _ := tenancy.LocationID(r.Context())
		line, err := deps.service.UpdatePurchaseOrderLine(r.Context(), purchasing.PurchaseOrderLineInput{
			ID:              r.PathValue("line_id"),
			LocationID:      locationID,
			PurchaseOrderID: r.PathValue("id"),
			VendorItemID:    payload.VendorItemID,
			OrderedQuantity: payload.OrderedQuantity,
			OrderedUnitCost: payload.OrderedUnitCost,
			Currency:        payload.Currency,
		})
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"line": toPurchaseOrderLineResponse(line), "request_id": RequestID(r.Context())})
	})))

	mux.Handle("DELETE /purchase-orders/{id}/lines/{line_id}", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		if err := deps.service.RemovePurchaseOrderLine(r.Context(), locationID, r.PathValue("id"), r.PathValue("line_id")); err != nil {
			writePurchasingError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"request_id": RequestID(r.Context())})
	})))

	mux.Handle("POST /purchase-orders/{id}/submit", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		order, err := deps.service.SubmitPurchaseOrder(r.Context(), locationID, r.PathValue("id"))
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"purchase_order": toPurchaseOrderResponse(order), "request_id": RequestID(r.Context())})
	})))

	mux.Handle("GET /receipts", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		receipts, err := deps.service.ListReceipts(r.Context(), locationID)
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		payload := make([]receiptResponse, len(receipts))
		for i, receipt := range receipts {
			payload[i] = toReceiptResponse(receipt)
		}
		WriteJSON(w, http.StatusOK, map[string]any{"receipts": payload, "request_id": RequestID(r.Context())})
	})))

	mux.Handle("POST /receipts", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload receiptRequest
		if err := decodePurchaseRequest(w, r, &payload); err != nil {
			writePurchaseDecodeError(w, r, err)
			return
		}
		locationID, _ := tenancy.LocationID(r.Context())
		receipt, err := deps.service.Receive(r.Context(), purchasing.ReceiptInput{
			ID:              payload.ID,
			LocationID:      locationID,
			PurchaseOrderID: payload.PurchaseOrderID,
			ReceivedAt:      payload.ReceivedAt,
			ReceivedBy:      payload.ReceivedBy,
			Notes:           payload.Notes,
			Lines:           toReceiptLineInputs(payload.Lines),
		})
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusCreated, map[string]any{"receipt": toReceiptResponse(receipt), "request_id": RequestID(r.Context())})
	})))

	mux.Handle("GET /receipts/{id}", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		receipt, err := deps.service.GetReceipt(r.Context(), locationID, r.PathValue("id"))
		if err != nil {
			writePurchasingError(w, r, err)
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{"receipt": toReceiptResponse(receipt), "request_id": RequestID(r.Context())})
	})))
}

func toVendorResponse(vendor purchasing.Vendor) vendorResponse {
	return vendorResponse{ID: vendor.ID, LocationID: vendor.LocationID, Name: vendor.Name, ExternalRef: vendor.ExternalRef, Status: vendor.Status}
}

func toVendorItemResponse(item purchasing.VendorItem) vendorItemResponse {
	return vendorItemResponse{
		ID:                     item.ID,
		LocationID:             item.LocationID,
		VendorID:               item.VendorID,
		IngredientID:           item.IngredientID,
		VendorSKU:              item.VendorSKU,
		Name:                   item.Name,
		PurchaseUnit:           item.PurchaseUnit,
		PackQuantity:           item.PackQuantity,
		IngredientBaseQuantity: item.IngredientBaseQuantity,
		LastUnitCost:           item.LastUnitCost.Float64(),
		Currency:               item.Currency,
		Status:                 item.Status,
	}
}

func toPurchaseOrderResponse(order purchasing.PurchaseOrder) purchaseOrderResponse {
	lines := make([]purchaseOrderLineResponse, len(order.Lines))
	for i, line := range order.Lines {
		lines[i] = toPurchaseOrderLineResponse(line)
	}
	return purchaseOrderResponse{
		ID:         order.ID,
		LocationID: order.LocationID,
		PONumber:   order.PONumber,
		VendorID:   order.VendorID,
		Status:     order.Status,
		OrderedAt:  order.OrderedAt,
		Notes:      order.Notes,
		Lines:      lines,
	}
}

func toPurchaseOrderLineResponse(line purchasing.PurchaseOrderLine) purchaseOrderLineResponse {
	return purchaseOrderLineResponse{
		ID:               line.ID,
		LocationID:       line.LocationID,
		PurchaseOrderID:  line.PurchaseOrderID,
		VendorItemID:     line.VendorItemID,
		OrderedQuantity:  line.OrderedQuantity,
		OrderedUnitCost:  line.OrderedUnitCost.Float64(),
		Currency:         line.Currency,
		ReceivedQuantity: line.ReceivedQuantity,
	}
}

func toReceiptResponse(receipt purchasing.Receipt) receiptResponse {
	lines := make([]receiptLineResponse, len(receipt.Lines))
	for i, line := range receipt.Lines {
		lines[i] = receiptLineResponse{
			ID:                  line.ID,
			LocationID:          line.LocationID,
			ReceiptID:           line.ReceiptID,
			PurchaseOrderLineID: line.PurchaseOrderLineID,
			IngredientID:        line.IngredientID,
			ReceivedQuantity:    line.ReceivedQuantity,
			ReceivedUnitCost:    line.ReceivedUnitCost.Float64(),
			Currency:            line.Currency,
			InventoryQuantity:   line.InventoryQuantity,
			InventoryUnit:       string(line.InventoryUnit),
		}
	}
	return receiptResponse{
		ID:              receipt.ID,
		LocationID:      receipt.LocationID,
		PurchaseOrderID: receipt.PurchaseOrderID,
		ReceivedAt:      receipt.ReceivedAt,
		ReceivedBy:      receipt.ReceivedBy,
		Notes:           receipt.Notes,
		Lines:           lines,
	}
}

func toReceiptLineInputs(lines []receiptLineRequest) []purchasing.ReceiptLineInput {
	out := make([]purchasing.ReceiptLineInput, len(lines))
	for i, line := range lines {
		out[i] = purchasing.ReceiptLineInput{
			ID:                  line.ID,
			PurchaseOrderLineID: line.PurchaseOrderLineID,
			ReceivedQuantity:    line.ReceivedQuantity,
			ReceivedUnitCost:    line.ReceivedUnitCost,
			Currency:            line.Currency,
		}
	}
	return out
}

func decodePurchaseRequest(w http.ResponseWriter, r *http.Request, payload any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxPurchaseRequestBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(payload); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return errRequestBodyTooLarge
		}
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain a single json object")
	}
	return nil
}

func writePurchaseDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errRequestBodyTooLarge) {
		WriteError(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds the 1 MiB limit")
		return
	}
	WriteError(w, r, http.StatusBadRequest, "invalid_json", "request body must be valid json")
}

func writePurchasingError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, purchasing.ErrInvalidPurchase):
		WriteError(w, r, http.StatusBadRequest, "invalid_purchase", err.Error())
	case errors.Is(err, purchasing.ErrVendorNotFound):
		WriteError(w, r, http.StatusNotFound, "vendor_not_found", "vendor not found")
	case errors.Is(err, purchasing.ErrVendorItemNotFound):
		WriteError(w, r, http.StatusNotFound, "vendor_item_not_found", "vendor item not found")
	case errors.Is(err, purchasing.ErrPurchaseOrderNotFound):
		WriteError(w, r, http.StatusNotFound, "purchase_order_not_found", "purchase order not found")
	case errors.Is(err, purchasing.ErrReceiptNotFound):
		WriteError(w, r, http.StatusNotFound, "receipt_not_found", "receipt not found")
	default:
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
