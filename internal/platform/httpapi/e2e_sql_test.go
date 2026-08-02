package httpapi

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ruth411/circle/internal/core/ingredient"
	"github.com/ruth411/circle/internal/core/recipe"
	"github.com/ruth411/circle/internal/diner"
	"github.com/ruth411/circle/internal/identity"
	"github.com/ruth411/circle/internal/inventory"
	"github.com/ruth411/circle/internal/ordering"
	"github.com/ruth411/circle/internal/platform/db"
	"github.com/ruth411/circle/internal/platform/events"
	"github.com/ruth411/circle/internal/platform/resolve"
	"github.com/ruth411/circle/internal/provisioning"
	"github.com/ruth411/circle/internal/purchasing"
	"github.com/ruth411/circle/internal/tenancy"
	projectmigrations "github.com/ruth411/circle/migrations"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var integrationSchemas = []string{
	"platform",
	"diner",
	"inventory",
	"ordering",
	"purchasing",
	"recipe",
	"ingredient",
	"identity",
	"tenancy",
}

func TestFreshLocationEndToEndFlowSQL(t *testing.T) {
	dsn := os.Getenv("CIRCLE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CIRCLE_TEST_DATABASE_URL to run SQL integration tests")
	}

	database := openIntegrationDatabase(t, dsn)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	suffix := fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	ingredientID := "ing-sql-chicken-a4-" + suffix
	recipeID := "rec-sql-chicken-bowl-a4-" + suffix
	menuItemID := "item-sql-chicken-bowl-a4-" + suffix
	snapshotID := "snap-sql-a4-1-" + suffix
	orderID := "order-sql-a4-1-" + suffix
	lineID := "line-sql-a4-1-" + suffix
	tenderID := "tender-sql-a4-1-" + suffix

	provisioned := provisionIntegrationLocation(t, database, suffix)

	outbox := events.NewSQLOutbox(database)
	inventoryService := inventory.NewService(inventory.NewSQLRepository(database))
	inventoryProcessor := inventory.NewProcessor(outbox, inventoryService)
	dinerService := diner.NewServiceWithRepository(diner.NewSQLRepository(database))
	dinerProcessor := diner.NewProcessor(outbox, dinerService)
	ingredientRepository := ingredient.NewSQLRepository(database)
	recipeRepository := recipe.NewSQLRepository(database)
	catalogService := recipe.NewCatalogService(recipeRepository, recipeRepository, ingredientRepository, resolve.SnapshotResolver{
		Recipes:     recipeRepository,
		Ingredients: ingredientRepository,
		MaxDepth:    recipe.DefaultMaxDepth,
	})
	server := NewServerWithDependencies(logger, Dependencies{
		IngredientService: ingredient.NewService(ingredientRepository),
		RecipeService:     recipe.NewService(recipeRepository, ingredientRepository),
		CatalogService: catalogService,
		DinerService:         dinerService,
		OrderingService:      ordering.NewServiceWithDependencies(ordering.NewSQLRepository(database), recipeRepository, ordering.MockProvider{}),
		PurchasingService:    purchasing.NewService(purchasing.NewSQLRepository(database), ingredientRepository),
		SessionValidator:     identity.NewSQLSessionValidator(database),
		OrganizationResolver: tenancy.NewSQLOrganizationResolver(database),
	})

	locationID := provisioned.LocationID
	sessionID := provisioned.Session.ID

	createIngredient := apiRequestWithSession(t, server, http.MethodPost, "/ingredients", locationID, sessionID, `{
		"id":"`+ingredientID+`",
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

	createRecipe := apiRequestWithSession(t, server, http.MethodPost, "/recipes", locationID, sessionID, `{
		"id":"`+recipeID+`",
		"name":"Chicken Bowl",
		"yield_count":1,
		"lines":[{"target_type":"ingredient","target_id":"`+ingredientID+`","quantity":150,"unit":"g","prep_method":"grilled"}]
	}`)
	if createRecipe.Code != http.StatusCreated {
		t.Fatalf("recipe status = %d, want 201, body = %s", createRecipe.Code, createRecipe.Body.String())
	}

	createMenuItem := apiRequestWithSession(t, server, http.MethodPost, "/menu-items", locationID, sessionID, `{
		"id":"`+menuItemID+`",
		"recipe_id":"`+recipeID+`",
		"name":"Chicken Bowl",
		"description":"SQL fresh-location bowl",
		"price_minor":1299,
		"currency":"USD",
		"modifier_groups":[]
	}`)
	if createMenuItem.Code != http.StatusCreated {
		t.Fatalf("menu item status = %d, want 201, body = %s", createMenuItem.Code, createMenuItem.Body.String())
	}

	createSnapshot := apiRequestWithSession(t, server, http.MethodPost, "/menu-snapshots", locationID, sessionID, `{"id":"`+snapshotID+`"}`)
	if createSnapshot.Code != http.StatusCreated {
		_, directErr := catalogService.GenerateSnapshot(context.Background(), recipe.GenerateSnapshotInput{
			ID:         "snap-sql-a4-direct-" + suffix,
			LocationID: locationID,
		})
		t.Fatalf("snapshot status = %d, want 201, body = %s, direct_generate_err = %v", createSnapshot.Code, createSnapshot.Body.String(), directErr)
	}

	createOrder := apiRequestWithSession(t, server, http.MethodPost, "/orders", locationID, sessionID, `{
		"id":"`+orderID+`",
		"snapshot_id":"`+snapshotID+`",
		"business_date":"2026-08-02"
	}`)
	if createOrder.Code != http.StatusCreated {
		t.Fatalf("order status = %d, want 201, body = %s", createOrder.Code, createOrder.Body.String())
	}

	addLine := apiRequestWithSession(t, server, http.MethodPost, "/orders/"+orderID+"/lines", locationID, sessionID, `{
		"line_id":"`+lineID+`",
		"menu_item_id":"`+menuItemID+`",
		"quantity":1
	}`)
	if addLine.Code != http.StatusCreated {
		t.Fatalf("line status = %d, want 201, body = %s", addLine.Code, addLine.Body.String())
	}

	closeOrder := apiRequestWithSession(t, server, http.MethodPost, "/orders/"+orderID+"/close", locationID, sessionID, `{
		"tender":{
			"id":"`+tenderID+`",
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

	token, err := dinerService.ResolveTokenByOrder(context.Background(), locationID, orderID)
	if err != nil {
		t.Fatalf("ResolveTokenByOrder returned error: %v", err)
	}

	getToken := apiRequestWithSession(t, server, http.MethodGet, "/diner/tokens/"+token.Token, "", "", "")
	if getToken.Code != http.StatusOK {
		t.Fatalf("token route status = %d, want 200, body = %s", getToken.Code, getToken.Body.String())
	}

	createClaim := apiRequestWithSession(t, server, http.MethodPost, "/diner/claims", "", "", `{
		"token":"`+token.Token+`",
		"selected_item_ids":["`+token.Items[0].ItemID+`"]
	}`)
	if createClaim.Code != http.StatusCreated {
		t.Fatalf("claim status = %d, want 201, body = %s", createClaim.Code, createClaim.Body.String())
	}
}

func TestGenerateSnapshotSQL(t *testing.T) {
	dsn := os.Getenv("CIRCLE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CIRCLE_TEST_DATABASE_URL to run SQL integration tests")
	}

	database := openIntegrationDatabase(t, dsn)
	suffix := fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	provisioned := provisionIntegrationLocation(t, database, suffix)

	ingredientRepository := ingredient.NewSQLRepository(database)
	recipeRepository := recipe.NewSQLRepository(database)
	catalogService := recipe.NewCatalogService(recipeRepository, recipeRepository, ingredientRepository, resolve.SnapshotResolver{
		Recipes:     recipeRepository,
		Ingredients: ingredientRepository,
		MaxDepth:    recipe.DefaultMaxDepth,
	})

	ctx := context.Background()
	locationID := provisioned.LocationID

	if _, err := ingredient.NewService(ingredientRepository).Create(ctx, ingredient.UpsertInput{
		ID:         "ing-sql-direct-chicken-" + suffix,
		LocationID: locationID,
		Name:       "Chicken",
		Category:   "protein",
		BaseUnit:   ingredient.UnitGram,
		MacrosPerBaseUnit: ingredient.MacroValues{
			Calories:     1,
			ProteinGrams: 0.1,
			CarbsGrams:   0,
			FatGrams:     0.02,
		},
		CurrentCostPerBaseUnit: 0.0123,
		Currency:               "USD",
		OnHandBaseUnits:        1000,
		ParLevelBaseUnits:      500,
		Provenance:             ingredient.ProvenanceRestaurantOfficial,
		VerificationStatus:     ingredient.VerificationVerified,
		ServingSizeQuantity:    100,
		ServingSizeUnit:        string(ingredient.UnitGram),
	}); err != nil {
		t.Fatalf("ingredient create returned error: %v", err)
	}

	if _, err := recipe.NewService(recipeRepository, ingredientRepository).Create(ctx, recipe.UpsertInput{
		ID:         "rec-sql-direct-bowl-" + suffix,
		LocationID: locationID,
		Name:       "Chicken Bowl",
		YieldCount: 1,
		Lines: []recipe.RecipeLine{{
			TargetType: recipe.LineTargetIngredient,
			TargetID:   "ing-sql-direct-chicken-" + suffix,
			Quantity:   150,
			Unit:       ingredient.UnitGram,
			PrepMethod: "grilled",
		}},
	}); err != nil {
		t.Fatalf("recipe create returned error: %v", err)
	}

	if _, err := catalogService.CreateMenuItem(ctx, recipe.MenuItem{
		ID:          "item-sql-direct-bowl-" + suffix,
		LocationID:  locationID,
		RecipeID:    "rec-sql-direct-bowl-" + suffix,
		Name:        "Chicken Bowl",
		Description: "Direct SQL fresh-location bowl",
		PriceMinor:  1299,
		Currency:    "USD",
	}); err != nil {
		t.Fatalf("menu item create returned error: %v", err)
	}

	snapshot, err := catalogService.GenerateSnapshot(ctx, recipe.GenerateSnapshotInput{
		ID:         "snap-sql-direct-" + suffix,
		LocationID: locationID,
	})
	if err != nil {
		t.Fatalf("GenerateSnapshot returned error: %v", err)
	}
	if snapshot.Version != 1 {
		t.Fatalf("snapshot version = %d, want 1", snapshot.Version)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("snapshot item count = %d, want 1", len(snapshot.Items))
	}
}

func openIntegrationDatabase(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	database, err := db.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("db.Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		t.Fatalf("PingContext returned error: %v", err)
	}

	migrations, err := db.LoadMigrationsFS(projectmigrations.FS, ".")
	if err != nil {
		t.Fatalf("LoadMigrationsFS returned error: %v", err)
	}
	if err := resetIntegrationDatabase(ctx, database); err != nil {
		t.Fatalf("resetIntegrationDatabase returned error: %v", err)
	}
	if err := db.ApplyMigrations(ctx, database.ExecContext, migrations); err != nil {
		t.Fatalf("ApplyMigrations returned error: %v", err)
	}

	return database
}

func resetIntegrationDatabase(ctx context.Context, database *sql.DB) error {
	for _, schema := range integrationSchemas {
		if _, err := database.ExecContext(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); err != nil {
			return err
		}
	}
	return nil
}

func provisionIntegrationLocation(t *testing.T, database *sql.DB, suffix string) provisioning.ProvisionedLocation {
	t.Helper()

	service := provisioning.NewService(provisioning.NewSQLRepository(database), func() time.Time {
		return time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	})
	provisioned, err := service.ProvisionLocation(context.Background(), provisioning.ProvisionLocationInput{
		OrganizationID:   "org-sql-a4-" + suffix,
		OrganizationName: "SQL A4 Org " + suffix,
		RestaurantID:     "rest-sql-a4-" + suffix,
		RestaurantName:   "SQL A4 Restaurant " + suffix,
		LocationID:       "loc-sql-a4-" + suffix,
		LocationName:     "SQL A4 Location " + suffix,
		TimezoneName:     "America/New_York",
		Currency:         "USD",
		RoleID:           "role-sql-a4-manager-" + suffix,
		RoleName:         "store_manager",
		UserID:           "user-sql-a4-manager-" + suffix,
		Email:            "sql-a4-manager-" + suffix + "@example.com",
		DisplayName:      "SQL A4 Manager",
		PasswordHash:     "seed-password-hash-change-me",
		SessionID:        "session-sql-a4-manager-" + suffix,
		SessionTTL:       24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("ProvisionLocation returned error: %v", err)
	}
	return provisioned
}

func apiRequestWithSession(t *testing.T, server http.Handler, method string, path string, locationID string, sessionID string, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if locationID != "" {
		request.Header.Set("X-Location-Id", locationID)
	}
	if sessionID != "" {
		request.Header.Set(sessionIDHeader, sessionID)
	}
	result := httptest.NewRecorder()
	server.ServeHTTP(result, request)
	return result
}
