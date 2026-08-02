package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/ruth411/circle/internal/identity"
	"github.com/ruth411/circle/internal/inventory"
	"github.com/ruth411/circle/internal/tenancy"
)

type inventoryDependencies struct {
	service              *inventory.Service
	locationResolver     tenancy.Resolver
	organizationResolver tenancy.OrganizationResolver
	sessionValidator     SessionValidator
}

type inventoryMovementResponse struct {
	ID           string    `json:"id"`
	LocationID   string    `json:"location_id"`
	SourceType   string    `json:"source_type"`
	SourceID     string    `json:"source_id"`
	OrderID      string    `json:"order_id,omitempty"`
	IngredientID string    `json:"ingredient_id"`
	Quantity     float64   `json:"quantity"`
	Unit         string    `json:"unit"`
	OccurredAt   time.Time `json:"occurred_at"`
}

type inventoryOnHandResponse struct {
	LocationID     string  `json:"location_id"`
	IngredientID   string  `json:"ingredient_id"`
	IngredientName string  `json:"ingredient_name"`
	BaseUnit       string  `json:"base_unit"`
	OnHandQuantity float64 `json:"on_hand_quantity"`
}

func registerInventoryRoutes(mux *http.ServeMux, deps inventoryDependencies) {
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

	mux.Handle("GET /inventory/movements", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		movements, err := deps.service.Movements(r.Context(), locationID)
		if err != nil {
			writeInventoryError(w, r, err)
			return
		}
		payload := make([]inventoryMovementResponse, len(movements))
		for i, movement := range movements {
			payload[i] = toInventoryMovementResponse(movement)
		}
		WriteJSON(w, http.StatusOK, map[string]any{"movements": payload, "request_id": RequestID(r.Context())})
	})))

	mux.Handle("GET /inventory/on-hand", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		items, err := deps.service.OnHand(r.Context(), locationID)
		if err != nil {
			writeInventoryError(w, r, err)
			return
		}
		payload := make([]inventoryOnHandResponse, len(items))
		for i, item := range items {
			payload[i] = toInventoryOnHandResponse(item)
		}
		WriteJSON(w, http.StatusOK, map[string]any{"items": payload, "request_id": RequestID(r.Context())})
	})))

	orgProtected := func(next http.Handler) http.Handler {
		return RequireOrganizationSession(deps.sessionValidator, next)
	}

	mux.Handle("GET /org/inventory/movements", orgProtected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := identity.SessionFromContext(r.Context())
		locationID := strings.TrimSpace(r.URL.Query().Get("location_id"))
		movements, err := deps.service.OrganizationMovements(r.Context(), session.OrganizationID, locationID)
		if err != nil {
			writeInventoryError(w, r, err)
			return
		}
		payload := make([]inventoryMovementResponse, len(movements))
		for i, movement := range movements {
			payload[i] = toInventoryMovementResponse(movement)
		}
		WriteJSON(w, http.StatusOK, map[string]any{"movements": payload, "request_id": RequestID(r.Context())})
	})))

	mux.Handle("GET /org/inventory/on-hand", orgProtected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := identity.SessionFromContext(r.Context())
		locationID := strings.TrimSpace(r.URL.Query().Get("location_id"))
		items, err := deps.service.OrganizationOnHand(r.Context(), session.OrganizationID, locationID)
		if err != nil {
			writeInventoryError(w, r, err)
			return
		}
		payload := make([]inventoryOnHandResponse, len(items))
		for i, item := range items {
			payload[i] = toInventoryOnHandResponse(item)
		}
		WriteJSON(w, http.StatusOK, map[string]any{"items": payload, "request_id": RequestID(r.Context())})
	})))
}

func toInventoryMovementResponse(movement inventory.Movement) inventoryMovementResponse {
	return inventoryMovementResponse{
		ID:           movement.ID,
		LocationID:   movement.LocationID,
		SourceType:   movement.SourceType,
		SourceID:     movement.SourceID,
		OrderID:      movement.OrderID,
		IngredientID: movement.IngredientID,
		Quantity:     movement.Quantity,
		Unit:         string(movement.Unit),
		OccurredAt:   movement.OccurredAt,
	}
}

func toInventoryOnHandResponse(item inventory.OnHandItem) inventoryOnHandResponse {
	return inventoryOnHandResponse{
		LocationID:     item.LocationID,
		IngredientID:   item.IngredientID,
		IngredientName: item.IngredientName,
		BaseUnit:       string(item.BaseUnit),
		OnHandQuantity: item.OnHandQuantity,
	}
}

func writeInventoryError(w http.ResponseWriter, r *http.Request, err error) {
	WriteError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
}
