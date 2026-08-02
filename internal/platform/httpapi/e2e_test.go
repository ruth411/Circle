package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ruth411/circle/internal/contracts"
	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/core/recipe"
	"github.com/ruth411/circle/internal/diner"
	"github.com/ruth411/circle/internal/inventory"
	"github.com/ruth411/circle/internal/ordering"
	"github.com/ruth411/circle/internal/platform/events"
	"github.com/ruth411/circle/internal/platform/resolve"
	"github.com/ruth411/circle/internal/tenancy"
)

func TestFreshLocationEndToEndFlow(t *testing.T) {
	t.Parallel()

	const (
		locationID = "loc-fresh-a4"
		orderID    = "order-a4-1"
		snapshotID = "snap-a4-1"
	)

	state := newPhaseA4State()
	outbox := &events.MemoryOutbox{}
	orderingService := ordering.NewServiceWithDependencies(
		newPhaseA4OrderingRepository(outbox),
		phaseA4SnapshotLookup{state: state},
		ordering.MockProvider{},
	)
	inventoryService := inventory.NewService(&phaseA4InventoryRepository{})
	inventoryProcessor := inventory.NewProcessor(outbox, inventoryService)
	dinerService := diner.NewService()
	dinerProcessor := diner.NewProcessor(outbox, dinerService)

	server := NewServerWithDependencies(slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		IngredientService: ingredient.NewService(phaseA4IngredientRepository{state: state}),
		RecipeService: recipe.NewService(
			phaseA4RecipeRepository{state: state},
			phaseA4IngredientLookup{state: state},
		),
		CatalogService: recipe.NewCatalogService(
			phaseA4CatalogRepository{state: state},
			phaseA4RecipeRepository{state: state},
			phaseA4IngredientLookup{state: state},
			resolve.SnapshotResolver{
				Recipes:     phaseA4RecipeRepository{state: state},
				Ingredients: phaseA4IngredientRepository{state: state},
				MaxDepth:    recipe.DefaultMaxDepth,
			},
		),
		OrderingService:      orderingService,
		DinerService:         dinerService,
		SessionValidator:     seedSessionService(t, locationID),
		OrganizationResolver: tenancy.StaticOrganizationResolver{locationID: "org-1"},
	})

	createIngredient := apiRequest(t, server, http.MethodPost, "/ingredients", locationID, `{
		"id":"ing-chicken-a4",
		"name":"Chicken",
		"category":"protein",
		"base_unit":"g",
		"macros_per_base_unit":{"calories":1,"protein_grams":0.1,"carbs_grams":0,"fat_grams":0.02},
		"current_cost_per_base_unit":0.0123,
		"currency":"USD",
		"on_hand_base_units":1000,
		"par_level_base_units":500,
		"provenance":"restaurant_official",
		"verification_status":"verified",
		"serving_size_quantity":100,
		"serving_size_unit":"g"
	}`)
	if createIngredient.Code != http.StatusCreated {
		t.Fatalf("ingredient status = %d, want 201, body = %s", createIngredient.Code, createIngredient.Body.String())
	}

	createRecipe := apiRequest(t, server, http.MethodPost, "/recipes", locationID, `{
		"id":"rec-chicken-bowl-a4",
		"name":"Chicken Bowl",
		"yield_count":1,
		"lines":[{"target_type":"ingredient","target_id":"ing-chicken-a4","quantity":150,"unit":"g","prep_method":"grilled"}]
	}`)
	if createRecipe.Code != http.StatusCreated {
		t.Fatalf("recipe status = %d, want 201, body = %s", createRecipe.Code, createRecipe.Body.String())
	}

	createMenuItem := apiRequest(t, server, http.MethodPost, "/menu-items", locationID, `{
		"id":"item-chicken-bowl-a4",
		"recipe_id":"rec-chicken-bowl-a4",
		"name":"Chicken Bowl",
		"description":"Fresh-location bowl",
		"price_minor":1299,
		"currency":"USD",
		"modifier_groups":[]
	}`)
	if createMenuItem.Code != http.StatusCreated {
		t.Fatalf("menu item status = %d, want 201, body = %s", createMenuItem.Code, createMenuItem.Body.String())
	}

	createSnapshot := apiRequest(t, server, http.MethodPost, "/menu-snapshots", locationID, `{"id":"`+snapshotID+`"}`)
	if createSnapshot.Code != http.StatusCreated {
		t.Fatalf("snapshot status = %d, want 201, body = %s", createSnapshot.Code, createSnapshot.Body.String())
	}
	var snapshotPayload struct {
		Snapshot struct {
			ID      string `json:"id"`
			Version int    `json:"version"`
			Items   []struct {
				MenuItemID string       `json:"menu_item_id"`
				Macros     macroPayload `json:"macros"`
			} `json:"items"`
		} `json:"snapshot"`
	}
	mustUnmarshal(t, createSnapshot.Body.Bytes(), &snapshotPayload)
	if snapshotPayload.Snapshot.ID != snapshotID {
		t.Fatalf("snapshot id = %q, want %q", snapshotPayload.Snapshot.ID, snapshotID)
	}
	if snapshotPayload.Snapshot.Version != 1 {
		t.Fatalf("snapshot version = %d, want 1", snapshotPayload.Snapshot.Version)
	}
	if len(snapshotPayload.Snapshot.Items) != 1 {
		t.Fatalf("snapshot item count = %d, want 1", len(snapshotPayload.Snapshot.Items))
	}
	if snapshotPayload.Snapshot.Items[0].Macros.Calories != 150 {
		t.Fatalf("snapshot calories = %v, want 150", snapshotPayload.Snapshot.Items[0].Macros.Calories)
	}

	createOrder := apiRequest(t, server, http.MethodPost, "/orders", locationID, `{
		"id":"`+orderID+`",
		"snapshot_id":"`+snapshotID+`",
		"business_date":"2026-08-02"
	}`)
	if createOrder.Code != http.StatusCreated {
		t.Fatalf("order status = %d, want 201, body = %s", createOrder.Code, createOrder.Body.String())
	}

	addLine := apiRequest(t, server, http.MethodPost, "/orders/"+orderID+"/lines", locationID, `{
		"line_id":"line-a4-1",
		"menu_item_id":"item-chicken-bowl-a4",
		"quantity":1
	}`)
	if addLine.Code != http.StatusCreated {
		t.Fatalf("line status = %d, want 201, body = %s", addLine.Code, addLine.Body.String())
	}
	var linePayload struct {
		Line struct {
			ResolvedPriceMinor int64              `json:"resolved_price_minor"`
			ResolvedMacros     macroPayload       `json:"resolved_macros"`
			IngredientUsage    map[string]float64 `json:"ingredient_usage"`
		} `json:"line"`
	}
	mustUnmarshal(t, addLine.Body.Bytes(), &linePayload)
	if linePayload.Line.ResolvedPriceMinor != 1299 {
		t.Fatalf("line price = %d, want 1299", linePayload.Line.ResolvedPriceMinor)
	}
	if linePayload.Line.ResolvedMacros.Calories != 150 {
		t.Fatalf("line calories = %v, want 150", linePayload.Line.ResolvedMacros.Calories)
	}
	if linePayload.Line.IngredientUsage["ing-chicken-a4"] != 150 {
		t.Fatalf("line usage = %v, want 150", linePayload.Line.IngredientUsage["ing-chicken-a4"])
	}

	closeOrder := apiRequest(t, server, http.MethodPost, "/orders/"+orderID+"/close", locationID, `{
		"tender":{
			"id":"tender-a4-1",
			"check_id":"`+orderID+`",
			"amount_minor":1299,
			"currency":"USD",
			"kind":"mock"
		}
	}`)
	if closeOrder.Code != http.StatusOK {
		t.Fatalf("close status = %d, want 200, body = %s", closeOrder.Code, closeOrder.Body.String())
	}

	if processed, err := inventoryProcessor.ProcessPendingClosedOrders(context.Background(), 10); err != nil {
		t.Fatalf("inventory processor returned error: %v", err)
	} else if processed != 1 {
		t.Fatalf("inventory processed = %d, want 1", processed)
	}
	if processed, err := dinerProcessor.ProcessPendingClosedOrders(context.Background(), 10); err != nil {
		t.Fatalf("diner processor returned error: %v", err)
	} else if processed != 1 {
		t.Fatalf("diner processed = %d, want 1", processed)
	}

	movements, err := inventoryService.Movements(context.Background(), locationID)
	if err != nil {
		t.Fatalf("inventory movements returned error: %v", err)
	}
	if len(movements) != 1 {
		t.Fatalf("movement count = %d, want 1", len(movements))
	}
	if movements[0].IngredientID != "ing-chicken-a4" {
		t.Fatalf("movement ingredient = %q, want ing-chicken-a4", movements[0].IngredientID)
	}
	if movements[0].Quantity != -150 {
		t.Fatalf("movement quantity = %v, want -150", movements[0].Quantity)
	}
	if movements[0].Unit != ingredient.UnitGram {
		t.Fatalf("movement unit = %s, want g", movements[0].Unit)
	}

	token, err := dinerService.ResolveTokenByOrder(context.Background(), locationID, orderID)
	if err != nil {
		t.Fatalf("ResolveTokenByOrder returned error: %v", err)
	}
	if len(token.Items) != 1 {
		t.Fatalf("token item count = %d, want 1", len(token.Items))
	}

	getToken := apiRequest(t, server, http.MethodGet, "/diner/tokens/"+token.Token, "", "")
	if getToken.Code != http.StatusOK {
		t.Fatalf("token route status = %d, want 200, body = %s", getToken.Code, getToken.Body.String())
	}
	var tokenPayload struct {
		Token struct {
			Token string `json:"token"`
			Items []struct {
				ItemID string       `json:"item_id"`
				Name   string       `json:"name"`
				Macros macroPayload `json:"macros"`
			} `json:"items"`
		} `json:"token"`
	}
	mustUnmarshal(t, getToken.Body.Bytes(), &tokenPayload)
	if tokenPayload.Token.Token != token.Token {
		t.Fatalf("public token = %q, want %q", tokenPayload.Token.Token, token.Token)
	}
	if tokenPayload.Token.Items[0].Macros.Calories != 150 {
		t.Fatalf("public token calories = %v, want 150", tokenPayload.Token.Items[0].Macros.Calories)
	}

	createClaim := apiRequest(t, server, http.MethodPost, "/diner/claims", "", `{
		"token":"`+token.Token+`",
		"selected_item_ids":["`+token.Items[0].ItemID+`"]
	}`)
	if createClaim.Code != http.StatusCreated {
		t.Fatalf("claim status = %d, want 201, body = %s", createClaim.Code, createClaim.Body.String())
	}
	var claimPayload struct {
		Claim struct {
			SelectedItemIDs []string     `json:"selected_item_ids"`
			Totals          macroPayload `json:"totals"`
		} `json:"claim"`
	}
	mustUnmarshal(t, createClaim.Body.Bytes(), &claimPayload)
	if len(claimPayload.Claim.SelectedItemIDs) != 1 {
		t.Fatalf("claim item count = %d, want 1", len(claimPayload.Claim.SelectedItemIDs))
	}
	if claimPayload.Claim.Totals.Calories != 150 {
		t.Fatalf("claim calories = %v, want 150", claimPayload.Claim.Totals.Calories)
	}
	if claimPayload.Claim.Totals.ProteinGrams != 15 {
		t.Fatalf("claim protein = %v, want 15", claimPayload.Claim.Totals.ProteinGrams)
	}
}

type phaseA4State struct {
	mu               sync.Mutex
	ingredients      map[string]ingredient.Ingredient
	recipes          map[string]recipe.Recipe
	menuItems        map[string]recipe.MenuItem
	snapshots        map[string]recipe.MenuSnapshot
	snapshotVersions map[string]int
}

func newPhaseA4State() *phaseA4State {
	return &phaseA4State{
		ingredients:      map[string]ingredient.Ingredient{},
		recipes:          map[string]recipe.Recipe{},
		menuItems:        map[string]recipe.MenuItem{},
		snapshots:        map[string]recipe.MenuSnapshot{},
		snapshotVersions: map[string]int{},
	}
}

type phaseA4IngredientRepository struct {
	state *phaseA4State
}

func (r phaseA4IngredientRepository) List(_ context.Context, locationID string, search string) ([]ingredient.Ingredient, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	var out []ingredient.Ingredient
	for _, item := range r.state.ingredients {
		if item.LocationID != locationID {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(item.Name), strings.ToLower(search)) {
			continue
		}
		out = append(out, item)
	}
	slices.SortFunc(out, func(a, b ingredient.Ingredient) int {
		return strings.Compare(a.ID, b.ID)
	})
	return out, nil
}

func (r phaseA4IngredientRepository) Create(_ context.Context, value ingredient.Ingredient) (ingredient.Ingredient, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	r.state.ingredients[phaseA4Key(value.LocationID, value.ID)] = value
	return value, nil
}

func (r phaseA4IngredientRepository) Update(_ context.Context, value ingredient.Ingredient) (ingredient.Ingredient, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	r.state.ingredients[phaseA4Key(value.LocationID, value.ID)] = value
	return value, nil
}

type phaseA4IngredientLookup struct {
	state *phaseA4State
}

func (l phaseA4IngredientLookup) Get(_ context.Context, locationID string, ingredientID string) (ingredient.Ingredient, error) {
	l.state.mu.Lock()
	defer l.state.mu.Unlock()

	value, ok := l.state.ingredients[phaseA4Key(locationID, ingredientID)]
	if !ok {
		return ingredient.Ingredient{}, ingredient.ErrNotFound
	}
	return value, nil
}

type phaseA4RecipeRepository struct {
	state *phaseA4State
}

func (r phaseA4RecipeRepository) Get(_ context.Context, locationID string, recipeID string) (recipe.Recipe, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	value, ok := r.state.recipes[phaseA4Key(locationID, recipeID)]
	if !ok {
		return recipe.Recipe{}, recipe.ErrRecipeNotFound
	}
	return value, nil
}

func (r phaseA4RecipeRepository) List(_ context.Context, locationID string) ([]recipe.Recipe, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	var out []recipe.Recipe
	for _, value := range r.state.recipes {
		if value.LocationID == locationID {
			out = append(out, value)
		}
	}
	slices.SortFunc(out, func(a, b recipe.Recipe) int {
		return strings.Compare(a.ID, b.ID)
	})
	return out, nil
}

func (r phaseA4RecipeRepository) Create(_ context.Context, value recipe.Recipe) (recipe.Recipe, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	r.state.recipes[phaseA4Key(value.LocationID, value.ID)] = value
	return value, nil
}

func (r phaseA4RecipeRepository) Update(_ context.Context, value recipe.Recipe) (recipe.Recipe, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	r.state.recipes[phaseA4Key(value.LocationID, value.ID)] = value
	return value, nil
}

type phaseA4CatalogRepository struct {
	state *phaseA4State
}

func (r phaseA4CatalogRepository) GetMenuItem(_ context.Context, locationID string, menuItemID string) (recipe.MenuItem, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	value, ok := r.state.menuItems[phaseA4Key(locationID, menuItemID)]
	if !ok {
		return recipe.MenuItem{}, recipe.ErrMenuItemNotFound
	}
	return value, nil
}

func (r phaseA4CatalogRepository) ListMenuItems(_ context.Context, locationID string) ([]recipe.MenuItem, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	var out []recipe.MenuItem
	for _, value := range r.state.menuItems {
		if value.LocationID == locationID {
			out = append(out, value)
		}
	}
	slices.SortFunc(out, func(a, b recipe.MenuItem) int {
		return strings.Compare(a.ID, b.ID)
	})
	return out, nil
}

func (r phaseA4CatalogRepository) CreateMenuItem(_ context.Context, value recipe.MenuItem) (recipe.MenuItem, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	r.state.menuItems[phaseA4Key(value.LocationID, value.ID)] = value
	return value, nil
}

func (r phaseA4CatalogRepository) UpdateMenuItem(_ context.Context, value recipe.MenuItem) (recipe.MenuItem, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	r.state.menuItems[phaseA4Key(value.LocationID, value.ID)] = value
	return value, nil
}

func (r phaseA4CatalogRepository) CreateSnapshot(_ context.Context, value recipe.MenuSnapshot) (recipe.MenuSnapshot, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	r.state.snapshotVersions[value.LocationID]++
	value.Version = r.state.snapshotVersions[value.LocationID]
	value.CreatedAt = time.Now().UTC()
	r.state.snapshots[phaseA4Key(value.LocationID, value.ID)] = value
	return value, nil
}

func (r phaseA4CatalogRepository) GetSnapshot(_ context.Context, locationID string, snapshotID string) (recipe.MenuSnapshot, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	value, ok := r.state.snapshots[phaseA4Key(locationID, snapshotID)]
	if !ok {
		return recipe.MenuSnapshot{}, recipe.ErrSnapshotNotFound
	}
	return value, nil
}

func (r phaseA4CatalogRepository) ListSnapshots(_ context.Context, locationID string) ([]recipe.MenuSnapshot, error) {
	r.state.mu.Lock()
	defer r.state.mu.Unlock()

	var out []recipe.MenuSnapshot
	for _, value := range r.state.snapshots {
		if value.LocationID == locationID {
			out = append(out, value)
		}
	}
	slices.SortFunc(out, func(a, b recipe.MenuSnapshot) int {
		return strings.Compare(a.ID, b.ID)
	})
	return out, nil
}

type phaseA4SnapshotLookup struct {
	state *phaseA4State
}

func (l phaseA4SnapshotLookup) GetSnapshot(_ context.Context, locationID string, snapshotID string) (recipe.MenuSnapshot, error) {
	l.state.mu.Lock()
	defer l.state.mu.Unlock()

	value, ok := l.state.snapshots[phaseA4Key(locationID, snapshotID)]
	if !ok {
		return recipe.MenuSnapshot{}, recipe.ErrSnapshotNotFound
	}
	return value, nil
}

type phaseA4OrderingRepository struct {
	mu      sync.Mutex
	orders  map[string]ordering.Order
	tenders map[string]phaseA4Tender
	outbox  events.Appender
}

type phaseA4Tender struct {
	OrderID string
	CheckID string
	Status  string
}

func newPhaseA4OrderingRepository(outbox events.Appender) *phaseA4OrderingRepository {
	return &phaseA4OrderingRepository{
		orders:  map[string]ordering.Order{},
		tenders: map[string]phaseA4Tender{},
		outbox:  outbox,
	}
}

func (r *phaseA4OrderingRepository) Get(_ context.Context, locationID string, orderID string) (ordering.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, ok := r.orders[orderID]
	if !ok || order.LocationID != locationID {
		return ordering.Order{}, ordering.ErrOrderNotFound
	}
	return clonePhaseA4Order(order), nil
}

func (r *phaseA4OrderingRepository) Create(_ context.Context, order ordering.Order) (ordering.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.orders[order.ID]; ok {
		if existing.CheckID == order.CheckID &&
			existing.LocationID == order.LocationID &&
			existing.SnapshotID == order.SnapshotID &&
			existing.SnapshotVersion == order.SnapshotVersion &&
			existing.BusinessDate == order.BusinessDate {
			return clonePhaseA4Order(existing), nil
		}
		return ordering.Order{}, ordering.ErrInvalidOrder
	}

	order.TotalMinor = 0
	r.orders[order.ID] = order
	return clonePhaseA4Order(order), nil
}

func (r *phaseA4OrderingRepository) AddLine(_ context.Context, locationID string, orderID string, line ordering.OrderLine) (ordering.OrderLine, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, ok := r.orders[orderID]
	if !ok || order.LocationID != locationID {
		return ordering.OrderLine{}, ordering.ErrOrderNotFound
	}
	if order.Status != ordering.OrderStatusOpen {
		return ordering.OrderLine{}, ordering.ErrOrderNotEditable
	}
	for _, existing := range order.Lines {
		if existing.LineID == line.LineID {
			return ordering.OrderLine{}, ordering.ErrInvalidOrder
		}
	}
	if order.Currency == "" {
		order.Currency = line.Currency
	}
	order.TotalMinor += line.ResolvedPriceMinor
	order.Lines = append(order.Lines, clonePhaseA4Line(line))
	r.orders[order.ID] = order
	return clonePhaseA4Line(line), nil
}

func (r *phaseA4OrderingRepository) StartClose(_ context.Context, locationID string, orderID string, tender ordering.Tender) (ordering.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, ok := r.orders[orderID]
	if !ok || order.LocationID != locationID {
		return ordering.Order{}, ordering.ErrOrderNotFound
	}
	if order.Status == ordering.OrderStatusClosed {
		return clonePhaseA4Order(order), nil
	}
	if order.Status == ordering.OrderStatusClosing {
		return ordering.Order{}, ordering.ErrOrderAlreadyClosing
	}
	if tender.AmountMinor < order.TotalMinor {
		return ordering.Order{}, ordering.ErrUnderpaidTender
	}

	order.Status = ordering.OrderStatusClosing
	r.orders[order.ID] = order
	r.tenders[phaseA4Key(locationID, tender.ID)] = phaseA4Tender{
		OrderID: order.ID,
		CheckID: order.CheckID,
		Status:  "pending",
	}
	return clonePhaseA4Order(order), nil
}

func (r *phaseA4OrderingRepository) MarkTenderSucceeded(_ context.Context, locationID string, orderID string, tenderID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := phaseA4Key(locationID, tenderID)
	tender, ok := r.tenders[key]
	if !ok || tender.OrderID != orderID {
		return ordering.ErrInvalidOrder
	}
	tender.Status = "succeeded"
	r.tenders[key] = tender
	return nil
}

func (r *phaseA4OrderingRepository) FailClose(_ context.Context, locationID string, orderID string, tenderID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, ok := r.orders[orderID]
	if !ok || order.LocationID != locationID {
		return ordering.ErrOrderNotFound
	}
	if order.Status == ordering.OrderStatusClosing {
		order.Status = ordering.OrderStatusOpen
		r.orders[order.ID] = order
	}
	if tender, ok := r.tenders[phaseA4Key(locationID, tenderID)]; ok {
		tender.Status = "failed"
		r.tenders[phaseA4Key(locationID, tenderID)] = tender
	}
	return nil
}

func (r *phaseA4OrderingRepository) FinishClose(_ context.Context, locationID string, orderID string, tenderID string, closedAt time.Time) (ordering.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, ok := r.orders[orderID]
	if !ok || order.LocationID != locationID {
		return ordering.Order{}, ordering.ErrOrderNotFound
	}
	tender, ok := r.tenders[phaseA4Key(locationID, tenderID)]
	if !ok || tender.OrderID != orderID || tender.Status != "succeeded" {
		return ordering.Order{}, ordering.ErrInvalidOrder
	}

	closedAt = closedAt.UTC()
	order.Status = ordering.OrderStatusClosed
	order.ClosedAt = &closedAt
	r.orders[order.ID] = order

	closedOrder, err := ordering.ToClosedOrder(order)
	if err != nil {
		return ordering.Order{}, err
	}
	payload, err := json.Marshal(closedOrder)
	if err != nil {
		return ordering.Order{}, err
	}
	if r.outbox != nil {
		if err := r.outbox.Append(context.Background(), events.Event{
			ID:          "evt-order-closed-" + order.LocationID + "-" + order.ID,
			Name:        contracts.ClosedOrderEventName,
			AggregateID: order.ID,
			LocationID:  order.LocationID,
			Payload:     payload,
			OccurredAt:  closedAt,
		}); err != nil {
			return ordering.Order{}, err
		}
	}

	return clonePhaseA4Order(order), nil
}

type phaseA4InventoryRepository struct {
	mu        sync.Mutex
	recorded  map[string]bool
	movements []inventory.Movement
}

func (r *phaseA4InventoryRepository) RecordDepletion(_ context.Context, order contracts.ClosedOrder) ([]inventory.Movement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recorded == nil {
		r.recorded = map[string]bool{}
	}
	key := phaseA4Key(order.LocationID, order.OrderID)
	if r.recorded[key] {
		return nil, nil
	}

	usage := map[string]float64{}
	units := map[string]ingredient.Unit{}
	for _, line := range order.Lines {
		for ingredientID, quantity := range line.IngredientUsage {
			usage[ingredientID] += quantity
			units[ingredientID] = line.IngredientUnits[ingredientID]
		}
	}

	ingredientIDs := make([]string, 0, len(usage))
	for ingredientID := range usage {
		ingredientIDs = append(ingredientIDs, ingredientID)
	}
	slices.Sort(ingredientIDs)

	out := make([]inventory.Movement, 0, len(ingredientIDs))
	for i, ingredientID := range ingredientIDs {
		out = append(out, inventory.Movement{
			ID:           "closed-order-" + order.OrderID + "-" + string(rune('1'+i)),
			LocationID:   order.LocationID,
			SourceType:   "closed_order",
			SourceID:     order.OrderID,
			OrderID:      order.OrderID,
			IngredientID: ingredientID,
			Quantity:     -usage[ingredientID],
			Unit:         units[ingredientID],
			OccurredAt:   order.ClosedAt.UTC(),
		})
	}

	r.movements = append(r.movements, out...)
	r.recorded[key] = true
	return out, nil
}

func (r *phaseA4InventoryRepository) RecordReceipt(context.Context, contracts.PurchaseReceipt) ([]inventory.Movement, error) {
	return nil, nil
}

func (r *phaseA4InventoryRepository) ListMovements(_ context.Context, locationID string) ([]inventory.Movement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []inventory.Movement
	for _, movement := range r.movements {
		if movement.LocationID == locationID {
			out = append(out, movement)
		}
	}
	return out, nil
}

func clonePhaseA4Order(order ordering.Order) ordering.Order {
	cloned := order
	cloned.Lines = make([]ordering.OrderLine, len(order.Lines))
	for i, line := range order.Lines {
		cloned.Lines[i] = clonePhaseA4Line(line)
	}
	if order.ClosedAt != nil {
		closedAt := *order.ClosedAt
		cloned.ClosedAt = &closedAt
	}
	return cloned
}

func clonePhaseA4Line(line ordering.OrderLine) ordering.OrderLine {
	cloned := line
	cloned.IngredientUsage = cloneFloatMap(line.IngredientUsage)
	cloned.IngredientUnits = cloneUnitMap(line.IngredientUnits)
	cloned.SelectedModifiers = make([]recipe.SnapshotModifier, len(line.SelectedModifiers))
	for i, modifier := range line.SelectedModifiers {
		cloned.SelectedModifiers[i] = recipe.SnapshotModifier{
			ModifierID:      modifier.ModifierID,
			Name:            modifier.Name,
			PriceDeltaMinor: modifier.PriceDeltaMinor,
			Currency:        modifier.Currency,
			MacroDelta:      modifier.MacroDelta,
			IngredientUsage: cloneFloatMap(modifier.IngredientUsage),
			IngredientUnits: cloneUnitMap(modifier.IngredientUnits),
		}
	}
	return cloned
}

func cloneFloatMap(input map[string]float64) map[string]float64 {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]float64, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func cloneUnitMap(input map[string]ingredient.Unit) map[string]ingredient.Unit {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]ingredient.Unit, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func phaseA4Key(locationID string, id string) string {
	return locationID + "|" + id
}

func apiRequest(t *testing.T, server http.Handler, method string, path string, locationID string, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if locationID != "" {
		req.Header.Set("X-Location-Id", locationID)
		req.Header.Set(sessionIDHeader, "session-1")
	}

	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, req)
	return recorder
}

func mustUnmarshal(t *testing.T, raw []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
}
