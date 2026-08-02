package ingredient

import (
	"context"
	"errors"
	"testing"
)

type fakeRepository struct {
	getFn    func(context.Context, string, string) (Ingredient, error)
	listFn   func(context.Context, string, string) ([]Ingredient, error)
	createFn func(context.Context, Ingredient) (Ingredient, error)
	updateFn func(context.Context, Ingredient) (Ingredient, error)
}

func (f fakeRepository) Get(ctx context.Context, locationID string, ingredientID string) (Ingredient, error) {
	return f.getFn(ctx, locationID, ingredientID)
}

func (f fakeRepository) List(ctx context.Context, locationID string, search string) ([]Ingredient, error) {
	return f.listFn(ctx, locationID, search)
}

func (f fakeRepository) Create(ctx context.Context, ingredient Ingredient) (Ingredient, error) {
	return f.createFn(ctx, ingredient)
}

func (f fakeRepository) Update(ctx context.Context, ingredient Ingredient) (Ingredient, error) {
	return f.updateFn(ctx, ingredient)
}

func TestCreateNormalizesAndValidatesIngredient(t *testing.T) {
	repo := fakeRepository{
		createFn: func(_ context.Context, ingredient Ingredient) (Ingredient, error) {
			return ingredient, nil
		},
	}
	service := NewService(repo)

	ingredient, err := service.Create(context.Background(), UpsertInput{
		ID:                     " ing-1 ",
		LocationID:             " loc-1 ",
		SourceItemID:           " cmg-1 ",
		Name:                   " Chicken ",
		Category:               " protein ",
		BaseUnit:               UnitEach,
		AlternateUnits:         map[Unit]float64{UnitGram: 28.35},
		MacrosPerBaseUnit:      MacroValues{Calories: 180, ProteinGrams: 32, CarbsGrams: 0, FatGrams: 7},
		CurrentCostPerBaseUnit: 12.3456,
		Currency:               " usd ",
		OnHandBaseUnits:        12,
		ParLevelBaseUnits:      4,
		Provenance:             ProvenanceRestaurantOfficial,
		VerificationStatus:     VerificationVerified,
		ServingSizeQuantity:    4,
		ServingSizeUnit:        " oz ",
		YieldFactors:           map[string]float64{" cooked ": 0.84},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if ingredient.ID != "ing-1" {
		t.Fatalf("ID = %q, want ing-1", ingredient.ID)
	}
	if ingredient.LocationID != "loc-1" {
		t.Fatalf("LocationID = %q, want loc-1", ingredient.LocationID)
	}
	if ingredient.Currency != "USD" {
		t.Fatalf("Currency = %q, want USD", ingredient.Currency)
	}
	if ingredient.ServingSizeUnit != "oz" {
		t.Fatalf("ServingSizeUnit = %q, want oz", ingredient.ServingSizeUnit)
	}
	if ingredient.AlternateUnits[UnitGram] != 28.35 {
		t.Fatalf("alternate gram factor = %v, want 28.35", ingredient.AlternateUnits[UnitGram])
	}
	if ingredient.YieldFactors["cooked"] != 0.84 {
		t.Fatalf("yield factor = %v, want 0.84", ingredient.YieldFactors["cooked"])
	}
	if ingredient.CurrentCostPerBaseUnit.Float64() != 12.3456 {
		t.Fatalf("current cost per base unit = %v, want 12.3456", ingredient.CurrentCostPerBaseUnit.Float64())
	}
}

func TestCreateRejectsInvalidIngredient(t *testing.T) {
	service := NewService(fakeRepository{
		createFn: func(_ context.Context, ingredient Ingredient) (Ingredient, error) {
			return ingredient, nil
		},
	})

	_, err := service.Create(context.Background(), UpsertInput{
		ID:                  "ing-1",
		LocationID:          "loc-1",
		Name:                "Chicken",
		Category:            "protein",
		BaseUnit:            UnitEach,
		MacrosPerBaseUnit:   MacroValues{Calories: -1},
		Currency:            "USD",
		Provenance:          ProvenanceRestaurantOfficial,
		VerificationStatus:  VerificationVerified,
		ServingSizeQuantity: 4,
		ServingSizeUnit:     "oz",
	})
	if !errors.Is(err, ErrInvalidIngredient) {
		t.Fatalf("err = %v, want ErrInvalidIngredient", err)
	}
}

func TestCreateRoundsCostToFourDecimals(t *testing.T) {
	service := NewService(fakeRepository{
		createFn: func(_ context.Context, ingredient Ingredient) (Ingredient, error) {
			return ingredient, nil
		},
	})

	ingredient, err := service.Create(context.Background(), UpsertInput{
		ID:                     "ing-1",
		LocationID:             "loc-1",
		Name:                   "Chicken",
		Category:               "protein",
		BaseUnit:               UnitEach,
		MacrosPerBaseUnit:      MacroValues{Calories: 180},
		CurrentCostPerBaseUnit: 1.45678999,
		Currency:               "USD",
		Provenance:             ProvenanceRestaurantOfficial,
		VerificationStatus:     VerificationVerified,
		ServingSizeQuantity:    4,
		ServingSizeUnit:        "oz",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if got, want := ingredient.CurrentCostPerBaseUnit.Float64(), 1.4568; got != want {
		t.Fatalf("current cost per base unit = %v, want %v", got, want)
	}
}

func TestCreateAcceptsLargeFourDecimalCost(t *testing.T) {
	service := NewService(fakeRepository{
		createFn: func(_ context.Context, ingredient Ingredient) (Ingredient, error) {
			return ingredient, nil
		},
	})

	ingredient, err := service.Create(context.Background(), UpsertInput{
		ID:                     "ing-1",
		LocationID:             "loc-1",
		Name:                   "Lobster Tail",
		Category:               "protein",
		BaseUnit:               UnitEach,
		MacrosPerBaseUnit:      MacroValues{Calories: 180},
		CurrentCostPerBaseUnit: 12345.6789,
		Currency:               "USD",
		Provenance:             ProvenanceRestaurantOfficial,
		VerificationStatus:     VerificationVerified,
		ServingSizeQuantity:    1,
		ServingSizeUnit:        "each",
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if got, want := ingredient.CurrentCostPerBaseUnit.Float64(), 12345.6789; got != want {
		t.Fatalf("current cost per base unit = %v, want %v", got, want)
	}
}

func TestUpdateReturnsRepositoryErrors(t *testing.T) {
	service := NewService(fakeRepository{
		updateFn: func(_ context.Context, ingredient Ingredient) (Ingredient, error) {
			return Ingredient{}, ErrNotFound
		},
	})

	_, err := service.Update(context.Background(), UpsertInput{
		ID:                  "ing-1",
		LocationID:          "loc-1",
		Name:                "Chicken",
		Category:            "protein",
		BaseUnit:            UnitEach,
		MacrosPerBaseUnit:   MacroValues{Calories: 180},
		Currency:            "USD",
		Provenance:          ProvenanceRestaurantOfficial,
		VerificationStatus:  VerificationVerified,
		ServingSizeQuantity: 4,
		ServingSizeUnit:     "oz",
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestListTrimsSearch(t *testing.T) {
	var gotLocationID string
	var gotSearch string

	service := NewService(fakeRepository{
		listFn: func(_ context.Context, locationID string, search string) ([]Ingredient, error) {
			gotLocationID = locationID
			gotSearch = search
			return nil, nil
		},
	})

	if _, err := service.List(context.Background(), " loc-1 ", " chicken "); err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if gotLocationID != "loc-1" {
		t.Fatalf("locationID = %q, want loc-1", gotLocationID)
	}
	if gotSearch != "chicken" {
		t.Fatalf("search = %q, want chicken", gotSearch)
	}
}

func TestGetTrimsIdentifiers(t *testing.T) {
	var gotLocationID string
	var gotIngredientID string

	service := NewService(fakeRepository{
		getFn: func(_ context.Context, locationID string, ingredientID string) (Ingredient, error) {
			gotLocationID = locationID
			gotIngredientID = ingredientID
			return Ingredient{ID: ingredientID, LocationID: locationID}, nil
		},
	})

	_, err := service.Get(context.Background(), " loc-1 ", " ing-1 ")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}

	if gotLocationID != "loc-1" {
		t.Fatalf("locationID = %q, want loc-1", gotLocationID)
	}
	if gotIngredientID != "ing-1" {
		t.Fatalf("ingredientID = %q, want ing-1", gotIngredientID)
	}
}
