package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/tenancy"
)

const maxIngredientRequestBodyBytes int64 = 1 << 20

var errRequestBodyTooLarge = errors.New("request body too large")

type ingredientDependencies struct {
	service              *ingredient.Service
	locationResolver     tenancy.Resolver
	organizationResolver tenancy.OrganizationResolver
	sessionValidator     SessionValidator
}

type ingredientRequest struct {
	ID                  string                        `json:"id,omitempty"`
	SourceItemID        string                        `json:"source_item_id,omitempty"`
	Name                string                        `json:"name"`
	Category            string                        `json:"category"`
	BaseUnit            ingredient.Unit               `json:"base_unit"`
	MacrosPerBaseUnit   macroPayload                  `json:"macros_per_base_unit"`
	CurrentCostMinor    int64                         `json:"current_cost_minor"`
	Currency            string                        `json:"currency"`
	OnHandBaseUnits     float64                       `json:"on_hand_base_units"`
	ParLevelBaseUnits   float64                       `json:"par_level_base_units"`
	Provenance          ingredient.Provenance         `json:"provenance"`
	VerificationStatus  ingredient.VerificationStatus `json:"verification_status"`
	ServingSizeQuantity float64                       `json:"serving_size_quantity"`
	ServingSizeUnit     string                        `json:"serving_size_unit"`
	AlternateUnits      map[ingredient.Unit]float64   `json:"alternate_units,omitempty"`
	YieldFactors        map[string]float64            `json:"yield_factors,omitempty"`
}

type macroPayload struct {
	Calories     float64 `json:"calories"`
	ProteinGrams float64 `json:"protein_grams"`
	CarbsGrams   float64 `json:"carbs_grams"`
	FatGrams     float64 `json:"fat_grams"`
}

type ingredientResponse struct {
	ID                  string                        `json:"id"`
	LocationID          string                        `json:"location_id"`
	SourceItemID        string                        `json:"source_item_id,omitempty"`
	Name                string                        `json:"name"`
	Category            string                        `json:"category"`
	BaseUnit            ingredient.Unit               `json:"base_unit"`
	MacrosPerBaseUnit   macroPayload                  `json:"macros_per_base_unit"`
	CurrentCostMinor    int64                         `json:"current_cost_minor"`
	Currency            string                        `json:"currency"`
	OnHandBaseUnits     float64                       `json:"on_hand_base_units"`
	ParLevelBaseUnits   float64                       `json:"par_level_base_units"`
	Provenance          ingredient.Provenance         `json:"provenance"`
	VerificationStatus  ingredient.VerificationStatus `json:"verification_status"`
	LowConfidence       bool                          `json:"low_confidence"`
	ServingSizeQuantity float64                       `json:"serving_size_quantity"`
	ServingSizeUnit     string                        `json:"serving_size_unit"`
	AlternateUnits      map[ingredient.Unit]float64   `json:"alternate_units,omitempty"`
	YieldFactors        map[string]float64            `json:"yield_factors,omitempty"`
}

func registerIngredientRoutes(mux *http.ServeMux, deps ingredientDependencies) {
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

	mux.Handle("GET /ingredients", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		ingredients, err := deps.service.List(r.Context(), locationID, r.URL.Query().Get("q"))
		if err != nil {
			writeIngredientError(w, r, err)
			return
		}

		payload := make([]ingredientResponse, len(ingredients))
		for i, value := range ingredients {
			payload[i] = toIngredientResponse(value)
		}
		WriteJSON(w, http.StatusOK, map[string]any{
			"ingredients": payload,
			"request_id":  RequestID(r.Context()),
		})
	})))

	mux.Handle("POST /ingredients", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload ingredientRequest
		if err := decodeIngredientRequest(w, r, &payload); err != nil {
			writeIngredientDecodeError(w, r, err)
			return
		}

		locationID, _ := tenancy.LocationID(r.Context())
		created, err := deps.service.Create(r.Context(), payload.toUpsertInput(locationID, payload.ID))
		if err != nil {
			writeIngredientError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusCreated, map[string]any{
			"ingredient": toIngredientResponse(created),
			"request_id": RequestID(r.Context()),
		})
	})))

	mux.Handle("PUT /ingredients/{id}", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload ingredientRequest
		if err := decodeIngredientRequest(w, r, &payload); err != nil {
			writeIngredientDecodeError(w, r, err)
			return
		}

		locationID, _ := tenancy.LocationID(r.Context())
		updated, err := deps.service.Update(r.Context(), payload.toUpsertInput(locationID, r.PathValue("id")))
		if err != nil {
			writeIngredientError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"ingredient": toIngredientResponse(updated),
			"request_id": RequestID(r.Context()),
		})
	})))
}

func (r ingredientRequest) toUpsertInput(locationID string, fallbackID string) ingredient.UpsertInput {
	id := r.ID
	if id == "" {
		id = fallbackID
	}

	return ingredient.UpsertInput{
		ID:             id,
		LocationID:     locationID,
		SourceItemID:   r.SourceItemID,
		Name:           r.Name,
		Category:       r.Category,
		BaseUnit:       r.BaseUnit,
		AlternateUnits: r.AlternateUnits,
		MacrosPerBaseUnit: ingredient.MacroValues{
			Calories:     r.MacrosPerBaseUnit.Calories,
			ProteinGrams: r.MacrosPerBaseUnit.ProteinGrams,
			CarbsGrams:   r.MacrosPerBaseUnit.CarbsGrams,
			FatGrams:     r.MacrosPerBaseUnit.FatGrams,
		},
		CurrentCostMinor:    r.CurrentCostMinor,
		Currency:            r.Currency,
		OnHandBaseUnits:     r.OnHandBaseUnits,
		ParLevelBaseUnits:   r.ParLevelBaseUnits,
		Provenance:          r.Provenance,
		VerificationStatus:  r.VerificationStatus,
		ServingSizeQuantity: r.ServingSizeQuantity,
		ServingSizeUnit:     r.ServingSizeUnit,
		YieldFactors:        r.YieldFactors,
	}
}

func toIngredientResponse(value ingredient.Ingredient) ingredientResponse {
	return ingredientResponse{
		ID:           value.ID,
		LocationID:   value.LocationID,
		SourceItemID: value.SourceItemID,
		Name:         value.Name,
		Category:     value.Category,
		BaseUnit:     value.BaseUnit,
		MacrosPerBaseUnit: macroPayload{
			Calories:     value.MacrosPerBaseUnit.Calories,
			ProteinGrams: value.MacrosPerBaseUnit.ProteinGrams,
			CarbsGrams:   value.MacrosPerBaseUnit.CarbsGrams,
			FatGrams:     value.MacrosPerBaseUnit.FatGrams,
		},
		CurrentCostMinor:    value.CurrentCostMinor,
		Currency:            value.Currency,
		OnHandBaseUnits:     value.OnHandBaseUnits,
		ParLevelBaseUnits:   value.ParLevelBaseUnits,
		Provenance:          value.Provenance,
		VerificationStatus:  value.VerificationStatus,
		LowConfidence:       value.VerificationStatus != ingredient.VerificationVerified,
		ServingSizeQuantity: value.ServingSizeQuantity,
		ServingSizeUnit:     value.ServingSizeUnit,
		AlternateUnits:      value.AlternateUnits,
		YieldFactors:        value.YieldFactors,
	}
}

func writeIngredientError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ingredient.ErrInvalidIngredient):
		WriteError(w, r, http.StatusBadRequest, "invalid_ingredient", err.Error())
	case errors.Is(err, ingredient.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "ingredient_not_found", "ingredient not found")
	default:
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func decodeIngredientRequest(w http.ResponseWriter, r *http.Request, payload *ingredientRequest) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxIngredientRequestBodyBytes)

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

func writeIngredientDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errRequestBodyTooLarge) {
		WriteError(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds the 1 MiB limit")
		return
	}
	WriteError(w, r, http.StatusBadRequest, "invalid_json", "request body must be valid json")
}
