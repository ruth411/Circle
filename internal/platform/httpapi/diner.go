package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/diner"
)

const maxDinerRequestBodyBytes int64 = 1 << 20

type dinerDependencies struct {
	service *diner.Service
}

type dinerClaimRequest struct {
	Token           string   `json:"token"`
	SelectedItemIDs []string `json:"selected_item_ids"`
}

type dinerTokenResponse struct {
	Token     string                    `json:"token"`
	ExpiresAt time.Time                 `json:"expires_at"`
	Items     []dinerPublicItemResponse `json:"items"`
}

type dinerPublicItemResponse struct {
	ItemID string       `json:"item_id"`
	LineID string       `json:"line_id"`
	Name   string       `json:"name"`
	Macros macroPayload `json:"macros"`
}

type dinerClaimResponse struct {
	ID              string       `json:"id"`
	SelectedItemIDs []string     `json:"selected_item_ids"`
	Totals          macroPayload `json:"totals"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

func registerDinerRoutes(mux *http.ServeMux, deps dinerDependencies) {
	if deps.service == nil {
		return
	}

	mux.Handle("GET /diner/tokens/{token}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := deps.service.ResolveToken(r.Context(), r.PathValue("token"))
		if err != nil {
			writeDinerError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"token":                 toDinerTokenResponse(token),
			"nutrition_disclaimer":  diner.NutritionDisclaimer,
			"request_id":            RequestID(r.Context()),
		})
	}))

	mux.Handle("POST /diner/claims", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload dinerClaimRequest
		if err := decodeDinerRequest(w, r, &payload); err != nil {
			writeDinerDecodeError(w, r, err)
			return
		}

		claim, err := deps.service.SubmitClaim(r.Context(), "", payload.Token, payload.SelectedItemIDs)
		if err != nil {
			writeDinerError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusCreated, map[string]any{
			"claim":                 toDinerClaimResponse(claim),
			"nutrition_disclaimer":  diner.NutritionDisclaimer,
			"request_id":            RequestID(r.Context()),
		})
	}))

	mux.Handle("PUT /diner/claims/{id}", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload dinerClaimRequest
		if err := decodeDinerRequest(w, r, &payload); err != nil {
			writeDinerDecodeError(w, r, err)
			return
		}

		claim, err := deps.service.ReviseClaim(r.Context(), r.PathValue("id"), payload.Token, payload.SelectedItemIDs)
		if err != nil {
			writeDinerError(w, r, err)
			return
		}

		WriteJSON(w, http.StatusOK, map[string]any{
			"claim":                 toDinerClaimResponse(claim),
			"nutrition_disclaimer":  diner.NutritionDisclaimer,
			"request_id":            RequestID(r.Context()),
		})
	}))
}

func toDinerTokenResponse(token diner.ReceiptToken) dinerTokenResponse {
	items := make([]dinerPublicItemResponse, len(token.Items))
	for i, item := range token.Items {
		items[i] = dinerPublicItemResponse{
			ItemID: item.ItemID,
			LineID: item.LineID,
			Name:   item.Name,
			Macros: macrosToPayload(item.Macros),
		}
	}
	return dinerTokenResponse{
		Token:     token.Token,
		ExpiresAt: token.ExpiresAt,
		Items:     items,
	}
}

func toDinerClaimResponse(claim diner.Claim) dinerClaimResponse {
	return dinerClaimResponse{
		ID:              claim.ID,
		SelectedItemIDs: append([]string(nil), claim.SelectedItemIDs...),
		Totals:          macrosToPayload(claim.Totals),
		UpdatedAt: claim.UpdatedAt,
	}
}

func decodeDinerRequest(w http.ResponseWriter, r *http.Request, payload *dinerClaimRequest) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxDinerRequestBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(payload); err != nil {
		return err
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain a single json object")
	}
	return nil
}

func writeDinerDecodeError(w http.ResponseWriter, r *http.Request, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		WriteError(w, r, http.StatusRequestEntityTooLarge, "request_too_large", "request body exceeds the 1 MiB limit")
		return
	}
	WriteError(w, r, http.StatusBadRequest, "invalid_json", "request body must be valid json")
}

func writeDinerError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, diner.ErrTokenNotFound), errors.Is(err, diner.ErrTokenExpired):
		WriteError(w, r, http.StatusNotFound, "token_unavailable", "receipt token is unavailable")
	case errors.Is(err, diner.ErrClaimNotFound):
		WriteError(w, r, http.StatusNotFound, "claim_not_found", "claim not found")
	case errors.Is(err, diner.ErrClaimAlreadyExists), errors.Is(err, diner.ErrInvalidClaim):
		WriteError(w, r, http.StatusBadRequest, "invalid_claim", err.Error())
	default:
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func macrosToPayload(macros ingredient.MacroValues) macroPayload {
	return macroPayload{
		Calories:     macros.Calories,
		ProteinGrams: macros.ProteinGrams,
		CarbsGrams:   macros.CarbsGrams,
		FatGrams:     macros.FatGrams,
	}
}
