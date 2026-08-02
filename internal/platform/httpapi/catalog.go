package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/core/recipe"
	"github.com/ruth411/circle/internal/tenancy"
)

const maxCatalogRequestBodyBytes int64 = 1 << 20

type catalogDependencies struct {
	service              *recipe.CatalogService
	locationResolver     tenancy.Resolver
	organizationResolver tenancy.OrganizationResolver
	sessionValidator     SessionValidator
}

type menuItemRequest struct {
	ID             string                 `json:"id,omitempty"`
	RecipeID       string                 `json:"recipe_id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description,omitempty"`
	PriceMinor     int64                  `json:"price_minor"`
	Currency       string                 `json:"currency"`
	ModifierGroups []modifierGroupRequest `json:"modifier_groups"`
}

type modifierGroupRequest struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	SelectionMin       int               `json:"selection_min"`
	SelectionMax       int               `json:"selection_max"`
	Required           bool              `json:"required"`
	Exclusive          bool              `json:"exclusive"`
	DefaultModifierIDs []string          `json:"default_modifier_ids,omitempty"`
	Modifiers          []modifierRequest `json:"modifiers"`
}

type modifierRequest struct {
	ID               string                   `json:"id"`
	Name             string                   `json:"name"`
	PriceDeltaMinor  int64                    `json:"price_delta_minor"`
	Currency         string                   `json:"currency"`
	IngredientDeltas []ingredientDeltaRequest `json:"ingredient_deltas,omitempty"`
}

type ingredientDeltaRequest struct {
	IngredientID string          `json:"ingredient_id"`
	Quantity     float64         `json:"quantity"`
	Unit         ingredient.Unit `json:"unit"`
	PrepMethod   string          `json:"prep_method,omitempty"`
}

type menuItemResponse struct {
	ID             string                  `json:"id"`
	LocationID     string                  `json:"location_id"`
	RecipeID       string                  `json:"recipe_id"`
	Name           string                  `json:"name"`
	Description    string                  `json:"description,omitempty"`
	PriceMinor     int64                   `json:"price_minor"`
	Currency       string                  `json:"currency"`
	ModifierGroups []modifierGroupResponse `json:"modifier_groups"`
}

type modifierGroupResponse struct {
	ID                 string             `json:"id"`
	Name               string             `json:"name"`
	SelectionMin       int                `json:"selection_min"`
	SelectionMax       int                `json:"selection_max"`
	Required           bool               `json:"required"`
	Exclusive          bool               `json:"exclusive"`
	DefaultModifierIDs []string           `json:"default_modifier_ids,omitempty"`
	Modifiers          []modifierResponse `json:"modifiers"`
}

type modifierResponse struct {
	ID               string                    `json:"id"`
	Name             string                    `json:"name"`
	PriceDeltaMinor  int64                     `json:"price_delta_minor"`
	Currency         string                    `json:"currency"`
	IngredientDeltas []ingredientDeltaResponse `json:"ingredient_deltas,omitempty"`
}

type ingredientDeltaResponse struct {
	IngredientID string          `json:"ingredient_id"`
	Quantity     float64         `json:"quantity"`
	Unit         ingredient.Unit `json:"unit"`
	PrepMethod   string          `json:"prep_method,omitempty"`
}

type snapshotRequest struct {
	ID string `json:"id"`
}

type menuSnapshotSummaryResponse struct {
	ID         string `json:"id"`
	LocationID string `json:"location_id"`
	Version    int    `json:"version"`
	CreatedAt  string `json:"created_at,omitempty"`
}

type menuSnapshotResponse struct {
	ID         string                 `json:"id"`
	LocationID string                 `json:"location_id"`
	Version    int                    `json:"version"`
	CreatedAt  string                 `json:"created_at,omitempty"`
	Items      []snapshotItemResponse `json:"items"`
}

type snapshotItemResponse struct {
	MenuItemID      string                          `json:"menu_item_id"`
	Name            string                          `json:"name"`
	Description     string                          `json:"description,omitempty"`
	PriceMinor      int64                           `json:"price_minor"`
	Currency        string                          `json:"currency"`
	CostMinor       int64                           `json:"cost_minor"`
	LowConfidence   bool                            `json:"low_confidence"`
	Macros          macroPayload                    `json:"macros"`
	IngredientUsage map[string]float64              `json:"ingredient_usage,omitempty"`
	IngredientUnits map[string]ingredient.Unit      `json:"ingredient_units,omitempty"`
	ModifierGroups  []snapshotModifierGroupResponse `json:"modifier_groups"`
}

type snapshotModifierGroupResponse struct {
	GroupID            string                     `json:"group_id"`
	Name               string                     `json:"name"`
	SelectionMin       int                        `json:"selection_min"`
	SelectionMax       int                        `json:"selection_max"`
	Required           bool                       `json:"required"`
	Exclusive          bool                       `json:"exclusive"`
	DefaultModifierIDs []string                   `json:"default_modifier_ids,omitempty"`
	Modifiers          []snapshotModifierResponse `json:"modifiers"`
}

type snapshotModifierResponse struct {
	ModifierID      string                     `json:"modifier_id"`
	Name            string                     `json:"name"`
	PriceDeltaMinor int64                      `json:"price_delta_minor"`
	Currency        string                     `json:"currency"`
	CostMinor       int64                      `json:"cost_minor"`
	LowConfidence   bool                       `json:"low_confidence"`
	MacroDelta      macroPayload               `json:"macro_delta"`
	IngredientUsage map[string]float64         `json:"ingredient_usage,omitempty"`
	IngredientUnits map[string]ingredient.Unit `json:"ingredient_units,omitempty"`
}

func registerCatalogRoutes(mux *http.ServeMux, deps catalogDependencies) {
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

	mux.Handle("GET /menu-items", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		items, err := deps.service.ListMenuItems(r.Context(), locationID)
		if err != nil {
			writeCatalogError(w, r, err)
			return
		}

		payload := make([]menuItemResponse, len(items))
		for i, item := range items {
			payload[i] = toMenuItemResponse(item)
		}
		WriteJSON(w, http.StatusOK, map[string]any{
			"menu_items": payload,
			"request_id": RequestID(r.Context()),
		})
	})))

	mux.Handle("GET /menu-items/{id}", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		item, err := deps.service.GetMenuItem(r.Context(), locationID, r.PathValue("id"))
		if err != nil {
			writeCatalogError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"menu_item":  toMenuItemResponse(item),
			"request_id": RequestID(r.Context()),
		})
	})))

	mux.Handle("POST /menu-items", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload menuItemRequest
		if err := decodeCatalogRequest(w, r, &payload); err != nil {
			writeCatalogDecodeError(w, r, err)
			return
		}

		locationID, _ := tenancy.LocationID(r.Context())
		item, err := deps.service.CreateMenuItem(r.Context(), payload.toMenuItem(locationID, payload.ID))
		if err != nil {
			writeCatalogError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusCreated, map[string]any{
			"menu_item":  toMenuItemResponse(item),
			"request_id": RequestID(r.Context()),
		})
	})))

	mux.Handle("PUT /menu-items/{id}", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload menuItemRequest
		if err := decodeCatalogRequest(w, r, &payload); err != nil {
			writeCatalogDecodeError(w, r, err)
			return
		}
		if payload.ID != "" && payload.ID != r.PathValue("id") {
			writeCatalogError(w, r, fmt.Errorf("%w: body id must match path id", recipe.ErrInvalidMenuItem))
			return
		}

		locationID, _ := tenancy.LocationID(r.Context())
		item, err := deps.service.UpdateMenuItem(r.Context(), payload.toMenuItem(locationID, r.PathValue("id")))
		if err != nil {
			writeCatalogError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"menu_item":  toMenuItemResponse(item),
			"request_id": RequestID(r.Context()),
		})
	})))

	mux.Handle("POST /menu-snapshots", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload snapshotRequest
		if err := decodeSnapshotRequest(w, r, &payload); err != nil {
			writeCatalogDecodeError(w, r, err)
			return
		}

		locationID, _ := tenancy.LocationID(r.Context())
		snapshot, err := deps.service.GenerateSnapshot(r.Context(), recipe.GenerateSnapshotInput{
			ID:         payload.ID,
			LocationID: locationID,
		})
		if err != nil {
			writeCatalogError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusCreated, map[string]any{
			"snapshot":   toMenuSnapshotResponse(snapshot),
			"request_id": RequestID(r.Context()),
		})
	})))

	mux.Handle("GET /menu-snapshots", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		snapshots, err := deps.service.ListSnapshots(r.Context(), locationID)
		if err != nil {
			writeCatalogError(w, r, err)
			return
		}

		payload := make([]menuSnapshotSummaryResponse, len(snapshots))
		for i, snapshot := range snapshots {
			payload[i] = toMenuSnapshotSummaryResponse(snapshot)
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"snapshots":  payload,
			"request_id": RequestID(r.Context()),
		})
	})))

	mux.Handle("GET /menu-snapshots/{id}", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		snapshot, err := deps.service.GetSnapshot(r.Context(), locationID, r.PathValue("id"))
		if err != nil {
			writeCatalogError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"snapshot":   toMenuSnapshotResponse(snapshot),
			"request_id": RequestID(r.Context()),
		})
	})))
}

func (r menuItemRequest) toMenuItem(locationID string, fallbackID string) recipe.MenuItem {
	id := r.ID
	if id == "" {
		id = fallbackID
	}

	groups := make([]recipe.ModifierGroup, len(r.ModifierGroups))
	for i, group := range r.ModifierGroups {
		modifiers := make([]recipe.Modifier, len(group.Modifiers))
		for j, modifier := range group.Modifiers {
			deltas := make([]recipe.IngredientDelta, len(modifier.IngredientDeltas))
			for k, delta := range modifier.IngredientDeltas {
				deltas[k] = recipe.IngredientDelta{
					IngredientID: delta.IngredientID,
					Quantity:     delta.Quantity,
					Unit:         delta.Unit,
					PrepMethod:   delta.PrepMethod,
				}
			}

			modifiers[j] = recipe.Modifier{
				ID:               modifier.ID,
				Name:             modifier.Name,
				PriceDeltaMinor:  modifier.PriceDeltaMinor,
				Currency:         modifier.Currency,
				IngredientDeltas: deltas,
			}
		}

		groups[i] = recipe.ModifierGroup{
			ID:                 group.ID,
			Name:               group.Name,
			SelectionMin:       group.SelectionMin,
			SelectionMax:       group.SelectionMax,
			Required:           group.Required,
			Exclusive:          group.Exclusive,
			DefaultModifierIDs: append([]string(nil), group.DefaultModifierIDs...),
			Modifiers:          modifiers,
		}
	}

	return recipe.MenuItem{
		ID:             id,
		LocationID:     locationID,
		RecipeID:       r.RecipeID,
		Name:           r.Name,
		Description:    r.Description,
		PriceMinor:     r.PriceMinor,
		Currency:       r.Currency,
		ModifierGroups: groups,
	}
}

func toMenuItemResponse(item recipe.MenuItem) menuItemResponse {
	groups := make([]modifierGroupResponse, len(item.ModifierGroups))
	for i, group := range item.ModifierGroups {
		modifiers := make([]modifierResponse, len(group.Modifiers))
		for j, modifier := range group.Modifiers {
			deltas := make([]ingredientDeltaResponse, len(modifier.IngredientDeltas))
			for k, delta := range modifier.IngredientDeltas {
				deltas[k] = ingredientDeltaResponse{
					IngredientID: delta.IngredientID,
					Quantity:     delta.Quantity,
					Unit:         delta.Unit,
					PrepMethod:   delta.PrepMethod,
				}
			}

			modifiers[j] = modifierResponse{
				ID:               modifier.ID,
				Name:             modifier.Name,
				PriceDeltaMinor:  modifier.PriceDeltaMinor,
				Currency:         modifier.Currency,
				IngredientDeltas: deltas,
			}
		}

		groups[i] = modifierGroupResponse{
			ID:                 group.ID,
			Name:               group.Name,
			SelectionMin:       group.SelectionMin,
			SelectionMax:       group.SelectionMax,
			Required:           group.Required,
			Exclusive:          group.Exclusive,
			DefaultModifierIDs: append([]string(nil), group.DefaultModifierIDs...),
			Modifiers:          modifiers,
		}
	}

	return menuItemResponse{
		ID:             item.ID,
		LocationID:     item.LocationID,
		RecipeID:       item.RecipeID,
		Name:           item.Name,
		Description:    item.Description,
		PriceMinor:     item.PriceMinor,
		Currency:       item.Currency,
		ModifierGroups: groups,
	}
}

func toMenuSnapshotSummaryResponse(snapshot recipe.MenuSnapshot) menuSnapshotSummaryResponse {
	response := menuSnapshotSummaryResponse{
		ID:         snapshot.ID,
		LocationID: snapshot.LocationID,
		Version:    snapshot.Version,
	}
	if !snapshot.CreatedAt.IsZero() {
		response.CreatedAt = snapshot.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	return response
}

func toMenuSnapshotResponse(snapshot recipe.MenuSnapshot) menuSnapshotResponse {
	items := make([]snapshotItemResponse, len(snapshot.Items))
	for i, item := range snapshot.Items {
		groups := make([]snapshotModifierGroupResponse, len(item.ModifierGroups))
		for j, group := range item.ModifierGroups {
			modifiers := make([]snapshotModifierResponse, len(group.Modifiers))
			for k, modifier := range group.Modifiers {
				modifiers[k] = snapshotModifierResponse{
					ModifierID:      modifier.ModifierID,
					Name:            modifier.Name,
					PriceDeltaMinor: modifier.PriceDeltaMinor,
					Currency:        modifier.Currency,
					CostMinor:       modifier.CostMinor,
					LowConfidence:   modifier.LowConfidence,
					MacroDelta: macroPayload{
						Calories:     modifier.MacroDelta.Calories,
						ProteinGrams: modifier.MacroDelta.ProteinGrams,
						CarbsGrams:   modifier.MacroDelta.CarbsGrams,
						FatGrams:     modifier.MacroDelta.FatGrams,
					},
					IngredientUsage: modifier.IngredientUsage,
					IngredientUnits: modifier.IngredientUnits,
				}
			}

			groups[j] = snapshotModifierGroupResponse{
				GroupID:            group.GroupID,
				Name:               group.Name,
				SelectionMin:       group.SelectionMin,
				SelectionMax:       group.SelectionMax,
				Required:           group.Required,
				Exclusive:          group.Exclusive,
				DefaultModifierIDs: append([]string(nil), group.DefaultModifierIDs...),
				Modifiers:          modifiers,
			}
		}

		items[i] = snapshotItemResponse{
			MenuItemID:      item.MenuItemID,
			Name:            item.Name,
			Description:     item.Description,
			PriceMinor:      item.PriceMinor,
			Currency:        item.Currency,
			CostMinor:       item.CostMinor,
			LowConfidence:   item.LowConfidence,
			Macros:          macroPayload{Calories: item.Macros.Calories, ProteinGrams: item.Macros.ProteinGrams, CarbsGrams: item.Macros.CarbsGrams, FatGrams: item.Macros.FatGrams},
			IngredientUsage: item.IngredientUsage,
			IngredientUnits: item.IngredientUnits,
			ModifierGroups:  groups,
		}
	}

	response := menuSnapshotResponse{
		ID:         snapshot.ID,
		LocationID: snapshot.LocationID,
		Version:    snapshot.Version,
		Items:      items,
	}
	if !snapshot.CreatedAt.IsZero() {
		response.CreatedAt = snapshot.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	return response
}

func writeCatalogError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, recipe.ErrInvalidMenuItem):
		WriteError(w, r, http.StatusBadRequest, "invalid_menu_item", err.Error())
	case errors.Is(err, recipe.ErrMenuItemAlreadyExists):
		WriteError(w, r, http.StatusConflict, "menu_item_already_exists", "menu item already exists")
	case errors.Is(err, recipe.ErrSnapshotAlreadyExists):
		WriteError(w, r, http.StatusConflict, "snapshot_already_exists", "snapshot already exists")
	case errors.Is(err, recipe.ErrMenuItemNotFound):
		WriteError(w, r, http.StatusNotFound, "menu_item_not_found", "menu item not found")
	case errors.Is(err, recipe.ErrInvalidSnapshot):
		WriteError(w, r, http.StatusBadRequest, "invalid_snapshot", err.Error())
	case errors.Is(err, recipe.ErrSnapshotNotFound):
		WriteError(w, r, http.StatusNotFound, "snapshot_not_found", "snapshot not found")
	default:
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func decodeCatalogRequest(w http.ResponseWriter, r *http.Request, payload *menuItemRequest) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxCatalogRequestBodyBytes)

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

func decodeSnapshotRequest(w http.ResponseWriter, r *http.Request, payload *snapshotRequest) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxCatalogRequestBodyBytes)

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

func writeCatalogDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errRequestBodyTooLarge) {
		WriteError(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds the 1 MiB limit")
		return
	}
	WriteError(w, r, http.StatusBadRequest, "invalid_json", "request body must be valid json")
}
