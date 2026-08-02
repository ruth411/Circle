package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/ruth411/circle/internal/core/recipe"
	"github.com/ruth411/circle/internal/ordering"
	"github.com/ruth411/circle/internal/platform/biztime"
	"github.com/ruth411/circle/internal/tenancy"
)

const maxOrderRequestBodyBytes int64 = 1 << 20

type orderingDependencies struct {
	service              *ordering.Service
	locationResolver     tenancy.Resolver
	organizationResolver tenancy.OrganizationResolver
	sessionValidator     SessionValidator
}

type createOrderRequest struct {
	ID           string `json:"id"`
	CheckID      string `json:"check_id,omitempty"`
	SnapshotID   string `json:"snapshot_id"`
	BusinessDate string `json:"business_date"`
}

type addOrderLineRequest struct {
	LineID      string   `json:"line_id,omitempty"`
	MenuItemID  string   `json:"menu_item_id"`
	ModifierIDs []string `json:"modifier_ids,omitempty"`
	Quantity    int      `json:"quantity"`
}

type closeOrderRequest struct {
	Tender tenderPayload `json:"tender"`
}

type tenderPayload struct {
	ID          string `json:"id"`
	CheckID     string `json:"check_id,omitempty"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency,omitempty"`
	Kind        string `json:"kind"`
}

type orderResponse struct {
	ID              string               `json:"id"`
	CheckID         string               `json:"check_id"`
	LocationID      string               `json:"location_id"`
	SnapshotID      string               `json:"snapshot_id"`
	SnapshotVersion int                  `json:"snapshot_version"`
	BusinessDate    string               `json:"business_date"`
	Status          ordering.OrderStatus `json:"status"`
	TotalMinor      int64                `json:"total_minor"`
	TotalMacros     macroPayload         `json:"total_macros"`
	Currency        string               `json:"currency"`
	ClosedAt        *time.Time           `json:"closed_at,omitempty"`
	Lines           []orderLineResponse  `json:"lines"`
}

type orderLineResponse struct {
	LineID             string                     `json:"line_id"`
	MenuItemID         string                     `json:"menu_item_id"`
	Name               string                     `json:"name"`
	Quantity           int                        `json:"quantity"`
	ResolvedPriceMinor int64                      `json:"resolved_price_minor"`
	Currency           string                     `json:"currency"`
	ResolvedMacros     macroPayload               `json:"resolved_macros"`
	IngredientUsage    map[string]float64         `json:"ingredient_usage"`
	SelectedModifiers  []selectedModifierResponse `json:"selected_modifiers"`
}

type selectedModifierResponse struct {
	ModifierID      string             `json:"modifier_id"`
	Name            string             `json:"name"`
	PriceDeltaMinor int64              `json:"price_delta_minor"`
	Currency        string             `json:"currency"`
	MacroDelta      macroPayload       `json:"macro_delta"`
	IngredientUsage map[string]float64 `json:"ingredient_usage"`
}

func registerOrderingRoutes(mux *http.ServeMux, deps orderingDependencies) {
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

	mux.Handle("POST /orders", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload createOrderRequest
		if err := decodeOrderRequest(w, r, &payload); err != nil {
			writeOrderDecodeError(w, r, err)
			return
		}

		locationID, _ := tenancy.LocationID(r.Context())
		businessDate, err := biztime.Parse(payload.BusinessDate)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "invalid_order", err.Error())
			return
		}

		order, err := deps.service.CreateOrder(r.Context(), ordering.CreateOrderInput{
			OrderID:      payload.ID,
			CheckID:      payload.CheckID,
			LocationID:   locationID,
			SnapshotID:   payload.SnapshotID,
			BusinessDate: businessDate,
		})
		if err != nil {
			writeOrderingError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusCreated, map[string]any{
			"order":      toOrderResponse(order),
			"request_id": RequestID(r.Context()),
		})
	})))

	mux.Handle("GET /orders/{id}", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		order, err := deps.service.GetOrder(r.Context(), locationID, r.PathValue("id"))
		if err != nil {
			writeOrderingError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"order":      toOrderResponse(order),
			"request_id": RequestID(r.Context()),
		})
	})))

	mux.Handle("POST /orders/{id}/lines", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload addOrderLineRequest
		if err := decodeOrderRequest(w, r, &payload); err != nil {
			writeOrderDecodeError(w, r, err)
			return
		}

		locationID, _ := tenancy.LocationID(r.Context())
		line, err := deps.service.AddLine(r.Context(), ordering.AddLineInput{
			LocationID:  locationID,
			OrderID:     r.PathValue("id"),
			LineID:      payload.LineID,
			MenuItemID:  payload.MenuItemID,
			ModifierIDs: payload.ModifierIDs,
			Quantity:    payload.Quantity,
		})
		if err != nil {
			writeOrderingError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusCreated, map[string]any{
			"line":       toOrderLineResponse(line),
			"request_id": RequestID(r.Context()),
		})
	})))

	mux.Handle("POST /orders/{id}/close", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload closeOrderRequest
		if err := decodeOrderRequest(w, r, &payload); err != nil {
			writeOrderDecodeError(w, r, err)
			return
		}

		locationID, _ := tenancy.LocationID(r.Context())
		order, err := deps.service.CloseCheck(r.Context(), ordering.CloseCheckInput{
			LocationID: locationID,
			OrderID:    r.PathValue("id"),
			Tender: ordering.Tender{
				ID:          payload.Tender.ID,
				CheckID:     payload.Tender.CheckID,
				AmountMinor: payload.Tender.AmountMinor,
				Currency:    payload.Tender.Currency,
				Kind:        payload.Tender.Kind,
			},
		})
		if err != nil {
			writeOrderingError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"order":      toOrderResponse(order),
			"request_id": RequestID(r.Context()),
		})
	})))
}

func toOrderResponse(order ordering.Order) orderResponse {
	lines := make([]orderLineResponse, len(order.Lines))
	totalMacros := macroPayload{}
	for i, line := range order.Lines {
		lines[i] = toOrderLineResponse(line)
		totalMacros.Calories += line.ResolvedMacros.Calories
		totalMacros.ProteinGrams += line.ResolvedMacros.ProteinGrams
		totalMacros.CarbsGrams += line.ResolvedMacros.CarbsGrams
		totalMacros.FatGrams += line.ResolvedMacros.FatGrams
	}

	return orderResponse{
		ID:              order.ID,
		CheckID:         order.CheckID,
		LocationID:      order.LocationID,
		SnapshotID:      order.SnapshotID,
		SnapshotVersion: order.SnapshotVersion,
		BusinessDate:    order.BusinessDate.String(),
		Status:          order.Status,
		TotalMinor:      order.TotalMinor,
		TotalMacros:     totalMacros,
		Currency:        order.Currency,
		ClosedAt:        order.ClosedAt,
		Lines:           lines,
	}
}

func toOrderLineResponse(line ordering.OrderLine) orderLineResponse {
	modifiers := make([]selectedModifierResponse, len(line.SelectedModifiers))
	for i, modifier := range line.SelectedModifiers {
		modifiers[i] = selectedModifierResponse{
			ModifierID:      modifier.ModifierID,
			Name:            modifier.Name,
			PriceDeltaMinor: modifier.PriceDeltaMinor,
			Currency:        modifier.Currency,
			MacroDelta: macroPayload{
				Calories:     modifier.MacroDelta.Calories,
				ProteinGrams: modifier.MacroDelta.ProteinGrams,
				CarbsGrams:   modifier.MacroDelta.CarbsGrams,
				FatGrams:     modifier.MacroDelta.FatGrams,
			},
			IngredientUsage: modifier.IngredientUsage,
		}
	}

	return orderLineResponse{
		LineID:             line.LineID,
		MenuItemID:         line.MenuItemID,
		Name:               line.Name,
		Quantity:           line.Quantity,
		ResolvedPriceMinor: line.ResolvedPriceMinor,
		Currency:           line.Currency,
		ResolvedMacros: macroPayload{
			Calories:     line.ResolvedMacros.Calories,
			ProteinGrams: line.ResolvedMacros.ProteinGrams,
			CarbsGrams:   line.ResolvedMacros.CarbsGrams,
			FatGrams:     line.ResolvedMacros.FatGrams,
		},
		IngredientUsage:   line.IngredientUsage,
		SelectedModifiers: modifiers,
	}
}

func decodeOrderRequest(w http.ResponseWriter, r *http.Request, payload any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxOrderRequestBodyBytes)

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

func writeOrderDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errRequestBodyTooLarge) {
		WriteError(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds the 1 MiB limit")
		return
	}
	WriteError(w, r, http.StatusBadRequest, "invalid_json", "request body must be valid json")
}

func writeOrderingError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ordering.ErrOrderNotFound):
		WriteError(w, r, http.StatusNotFound, "order_not_found", "order not found")
	case errors.Is(err, recipe.ErrSnapshotNotFound):
		WriteError(w, r, http.StatusNotFound, "snapshot_not_found", "snapshot not found")
	case errors.Is(err, ordering.ErrOrderAlreadyClosing):
		WriteError(w, r, http.StatusConflict, "order_closing", "order is already closing")
	case errors.Is(err, ordering.ErrOrderNotEditable):
		WriteError(w, r, http.StatusConflict, "order_not_editable", "order is not editable")
	case errors.Is(err, ordering.ErrUnderpaidTender):
		WriteError(w, r, http.StatusBadRequest, "underpaid_tender", err.Error())
	case errors.Is(err, ordering.ErrPaymentFailed):
		WriteError(w, r, http.StatusConflict, "payment_failed", err.Error())
	case errors.Is(err, ordering.ErrInvalidOrder):
		WriteError(w, r, http.StatusBadRequest, "invalid_order", err.Error())
	default:
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
