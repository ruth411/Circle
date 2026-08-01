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

func writeCatalogError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, recipe.ErrInvalidMenuItem):
		WriteError(w, r, http.StatusBadRequest, "invalid_menu_item", err.Error())
	case errors.Is(err, recipe.ErrMenuItemNotFound):
		WriteError(w, r, http.StatusNotFound, "menu_item_not_found", "menu item not found")
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

func writeCatalogDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errRequestBodyTooLarge) {
		WriteError(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds the 1 MiB limit")
		return
	}
	WriteError(w, r, http.StatusBadRequest, "invalid_json", "request body must be valid json")
}
