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
	"time"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/diner"
)

func TestDinerResolveTokenRouteReturnsPublicToken(t *testing.T) {
	service := diner.NewService()
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	service.SetNowForTests(func() time.Time { return now })

	token, err := service.IssueToken(context.Background(), contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines: []contracts.ClosedOrderLine{
			{LineID: "line-1", Name: "Bowl", Quantity: 2, ResolvedMacros: ingredient.MacroValues{Calories: 600}},
		},
	})
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		DinerService: service,
	})
	req := httptest.NewRequest(http.MethodGet, "/diner/tokens/"+token.Token, nil)
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", recorder.Code, recorder.Body.String())
	}

	var payload struct {
		Token struct {
			Token     string `json:"token"`
			ExpiresAt string `json:"expires_at"`
			Items     []struct {
				ItemID string       `json:"item_id"`
				LineID string       `json:"line_id"`
				Name   string       `json:"name"`
				Macros macroPayload `json:"macros"`
			} `json:"items"`
		} `json:"token"`
		NutritionDisclaimer string `json:"nutrition_disclaimer"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Token.Token != token.Token {
		t.Fatalf("token = %q, want %q", payload.Token.Token, token.Token)
	}
	if len(payload.Token.Items) != 2 {
		t.Fatalf("item count = %d, want 2", len(payload.Token.Items))
	}
	if payload.NutritionDisclaimer == "" {
		t.Fatal("nutrition disclaimer empty, want value")
	}
}

func TestDinerClaimRoutesCreateAndReviseClaim(t *testing.T) {
	service := diner.NewService()
	now := time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)
	service.SetNowForTests(func() time.Time { return now })

	token, err := service.IssueToken(context.Background(), contracts.ClosedOrder{
		OrderID:    "order-1",
		LocationID: "loc-1",
		ClosedAt:   now,
		Lines: []contracts.ClosedOrderLine{
			{LineID: "line-1", Name: "Bowl", Quantity: 1, ResolvedMacros: ingredient.MacroValues{Calories: 600}},
			{LineID: "line-2", Name: "Cookie", Quantity: 1, ResolvedMacros: ingredient.MacroValues{Calories: 200}},
		},
	})
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		DinerService: service,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/diner/claims", bytes.NewBufferString(`{
		"token":"`+token.Token+`",
		"selected_item_ids":["`+token.Items[0].ItemID+`"]
	}`))
	createRecorder := httptest.NewRecorder()
	server.ServeHTTP(createRecorder, createReq)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201, body = %s", createRecorder.Code, createRecorder.Body.String())
	}

	var created struct {
		Claim struct {
			ID     string       `json:"id"`
			Totals macroPayload `json:"totals"`
		} `json:"claim"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("create Unmarshal returned error: %v", err)
	}
	if created.Claim.ID == "" {
		t.Fatal("claim id empty, want generated id")
	}
	if created.Claim.Totals.Calories != 600 {
		t.Fatalf("create calories = %v, want 600", created.Claim.Totals.Calories)
	}

	reviseReq := httptest.NewRequest(http.MethodPut, "/diner/claims/"+created.Claim.ID, bytes.NewBufferString(`{
		"token":"`+token.Token+`",
		"selected_item_ids":["`+token.Items[0].ItemID+`","`+token.Items[1].ItemID+`"]
	}`))
	reviseRecorder := httptest.NewRecorder()
	server.ServeHTTP(reviseRecorder, reviseReq)

	if reviseRecorder.Code != http.StatusOK {
		t.Fatalf("revise status = %d, want 200, body = %s", reviseRecorder.Code, reviseRecorder.Body.String())
	}

	var revised struct {
		Claim struct {
			Totals macroPayload `json:"totals"`
		} `json:"claim"`
		NutritionDisclaimer string `json:"nutrition_disclaimer"`
	}
	if err := json.Unmarshal(reviseRecorder.Body.Bytes(), &revised); err != nil {
		t.Fatalf("revise Unmarshal returned error: %v", err)
	}
	if revised.Claim.Totals.Calories != 800 {
		t.Fatalf("revise calories = %v, want 800", revised.Claim.Totals.Calories)
	}
	if revised.NutritionDisclaimer == "" {
		t.Fatal("nutrition disclaimer empty, want value")
	}
}

func TestDinerClaimRouteRejectsInvalidJSON(t *testing.T) {
	service := diner.NewService()
	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		DinerService: service,
	})

	req := httptest.NewRequest(http.MethodPost, "/diner/claims", bytes.NewBufferString(`{"token":"x"`))
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", recorder.Code, recorder.Body.String())
	}

	var payload ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if payload.Error.Code != "invalid_json" {
		t.Fatalf("error code = %q, want invalid_json", payload.Error.Code)
	}
}
