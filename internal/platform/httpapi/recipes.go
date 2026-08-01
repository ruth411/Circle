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

const maxRecipeRequestBodyBytes int64 = 1 << 20

type recipeDependencies struct {
	service              *recipe.Service
	locationResolver     tenancy.Resolver
	organizationResolver tenancy.OrganizationResolver
	sessionValidator     SessionValidator
}

type recipeRequest struct {
	ID         string              `json:"id,omitempty"`
	Name       string              `json:"name"`
	YieldCount float64             `json:"yield_count"`
	Lines      []recipeLineRequest `json:"lines"`
}

type recipeLineRequest struct {
	TargetType recipe.LineTargetType `json:"target_type"`
	TargetID   string                `json:"target_id"`
	Quantity   float64               `json:"quantity"`
	Unit       ingredient.Unit       `json:"unit"`
	PrepMethod string                `json:"prep_method,omitempty"`
}

type recipeResponse struct {
	ID         string             `json:"id"`
	LocationID string             `json:"location_id"`
	Name       string             `json:"name"`
	YieldCount float64            `json:"yield_count"`
	Lines      []recipeLineOutput `json:"lines"`
}

type recipeLineOutput struct {
	LineNumber int                   `json:"line_number"`
	TargetType recipe.LineTargetType `json:"target_type"`
	TargetID   string                `json:"target_id"`
	Quantity   float64               `json:"quantity"`
	Unit       ingredient.Unit       `json:"unit"`
	PrepMethod string                `json:"prep_method,omitempty"`
}

func registerRecipeRoutes(mux *http.ServeMux, deps recipeDependencies) {
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

	mux.Handle("GET /recipes", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		recipes, err := deps.service.List(r.Context(), locationID)
		if err != nil {
			writeRecipeError(w, r, err)
			return
		}

		payload := make([]recipeResponse, len(recipes))
		for i, value := range recipes {
			payload[i] = toRecipeResponse(value)
		}
		WriteJSON(w, http.StatusOK, map[string]any{
			"recipes":    payload,
			"request_id": RequestID(r.Context()),
		})
	})))

	mux.Handle("GET /recipes/{id}", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID, _ := tenancy.LocationID(r.Context())
		value, err := deps.service.Get(r.Context(), locationID, r.PathValue("id"))
		if err != nil {
			writeRecipeError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"recipe":     toRecipeResponse(value),
			"request_id": RequestID(r.Context()),
		})
	})))

	mux.Handle("POST /recipes", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload recipeRequest
		if err := decodeRecipeRequest(w, r, &payload); err != nil {
			writeRecipeDecodeError(w, r, err)
			return
		}

		locationID, _ := tenancy.LocationID(r.Context())
		created, err := deps.service.Create(r.Context(), payload.toUpsertInput(locationID, payload.ID))
		if err != nil {
			writeRecipeError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusCreated, map[string]any{
			"recipe":     toRecipeResponse(created),
			"request_id": RequestID(r.Context()),
		})
	})))

	mux.Handle("PUT /recipes/{id}", protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload recipeRequest
		if err := decodeRecipeRequest(w, r, &payload); err != nil {
			writeRecipeDecodeError(w, r, err)
			return
		}
		if payload.ID != "" && payload.ID != r.PathValue("id") {
			writeRecipeError(w, r, fmt.Errorf("%w: body id must match path id", recipe.ErrInvalidRecipe))
			return
		}

		locationID, _ := tenancy.LocationID(r.Context())
		updated, err := deps.service.Update(r.Context(), payload.toUpsertInput(locationID, r.PathValue("id")))
		if err != nil {
			writeRecipeError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"recipe":     toRecipeResponse(updated),
			"request_id": RequestID(r.Context()),
		})
	})))
}

func (r recipeRequest) toUpsertInput(locationID string, fallbackID string) recipe.UpsertInput {
	id := r.ID
	if id == "" {
		id = fallbackID
	}

	lines := make([]recipe.RecipeLine, len(r.Lines))
	for i, line := range r.Lines {
		lines[i] = recipe.RecipeLine{
			TargetType: line.TargetType,
			TargetID:   line.TargetID,
			Quantity:   line.Quantity,
			Unit:       line.Unit,
			PrepMethod: line.PrepMethod,
		}
	}

	return recipe.UpsertInput{
		ID:         id,
		LocationID: locationID,
		Name:       r.Name,
		YieldCount: r.YieldCount,
		Lines:      lines,
	}
}

func toRecipeResponse(value recipe.Recipe) recipeResponse {
	lines := make([]recipeLineOutput, len(value.Lines))
	for i, line := range value.Lines {
		lines[i] = recipeLineOutput{
			LineNumber: line.LineNumber,
			TargetType: line.TargetType,
			TargetID:   line.TargetID,
			Quantity:   line.Quantity,
			Unit:       line.Unit,
			PrepMethod: line.PrepMethod,
		}
	}

	return recipeResponse{
		ID:         value.ID,
		LocationID: value.LocationID,
		Name:       value.Name,
		YieldCount: value.YieldCount,
		Lines:      lines,
	}
}

func writeRecipeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, recipe.ErrInvalidRecipe):
		WriteError(w, r, http.StatusBadRequest, "invalid_recipe", err.Error())
	case errors.Is(err, recipe.ErrRecipeNotFound):
		WriteError(w, r, http.StatusNotFound, "recipe_not_found", "recipe not found")
	default:
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func decodeRecipeRequest(w http.ResponseWriter, r *http.Request, payload *recipeRequest) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRecipeRequestBodyBytes)

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

func writeRecipeDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errRequestBodyTooLarge) {
		WriteError(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds the 1 MiB limit")
		return
	}
	WriteError(w, r, http.StatusBadRequest, "invalid_json", "request body must be valid json")
}
