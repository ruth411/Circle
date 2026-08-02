package ingredient

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	ErrNotFound          = errors.New("ingredient not found")
	ErrInvalidIngredient = errors.New("invalid ingredient")
)

type Repository interface {
	Get(context.Context, string, string) (Ingredient, error)
	List(context.Context, string, string) ([]Ingredient, error)
	Create(context.Context, Ingredient) (Ingredient, error)
	Update(context.Context, Ingredient) (Ingredient, error)
}

type Service struct {
	repo Repository
}

type UpsertInput struct {
	ID                     string
	LocationID             string
	SourceItemID           string
	Name                   string
	Category               string
	BaseUnit               Unit
	AlternateUnits         map[Unit]float64
	MacrosPerBaseUnit      MacroValues
	CurrentCostPerBaseUnit float64
	Currency               string
	OnHandBaseUnits        float64
	ParLevelBaseUnits      float64
	Provenance             Provenance
	VerificationStatus     VerificationStatus
	ServingSizeQuantity    float64
	ServingSizeUnit        string
	YieldFactors           map[string]float64
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, locationID string, search string) ([]Ingredient, error) {
	if strings.TrimSpace(locationID) == "" {
		return nil, fmt.Errorf("%w: location id is required", ErrInvalidIngredient)
	}

	return s.repo.List(ctx, strings.TrimSpace(locationID), strings.TrimSpace(search))
}

func (s *Service) Get(ctx context.Context, locationID string, ingredientID string) (Ingredient, error) {
	if strings.TrimSpace(locationID) == "" {
		return Ingredient{}, fmt.Errorf("%w: location id is required", ErrInvalidIngredient)
	}
	if strings.TrimSpace(ingredientID) == "" {
		return Ingredient{}, fmt.Errorf("%w: ingredient id is required", ErrInvalidIngredient)
	}

	return s.repo.Get(ctx, strings.TrimSpace(locationID), strings.TrimSpace(ingredientID))
}

func (s *Service) Create(ctx context.Context, input UpsertInput) (Ingredient, error) {
	ingredient, err := normalizeIngredient(input, true)
	if err != nil {
		return Ingredient{}, err
	}

	return s.repo.Create(ctx, ingredient)
}

func (s *Service) Update(ctx context.Context, input UpsertInput) (Ingredient, error) {
	ingredient, err := normalizeIngredient(input, true)
	if err != nil {
		return Ingredient{}, err
	}

	return s.repo.Update(ctx, ingredient)
}

func normalizeIngredient(input UpsertInput, requireID bool) (Ingredient, error) {
	ingredient := Ingredient{
		ID:                     strings.TrimSpace(input.ID),
		LocationID:             strings.TrimSpace(input.LocationID),
		SourceItemID:           strings.TrimSpace(input.SourceItemID),
		Name:                   strings.TrimSpace(input.Name),
		Category:               strings.TrimSpace(input.Category),
		BaseUnit:               Unit(strings.TrimSpace(string(input.BaseUnit))),
		AlternateUnits:         normalizeAlternateUnits(input.AlternateUnits),
		MacrosPerBaseUnit:      input.MacrosPerBaseUnit,
		CurrentCostPerBaseUnit: NewCostPerBaseUnit(input.CurrentCostPerBaseUnit),
		Currency:               strings.ToUpper(strings.TrimSpace(input.Currency)),
		OnHandBaseUnits:        input.OnHandBaseUnits,
		ParLevelBaseUnits:      input.ParLevelBaseUnits,
		Provenance:             Provenance(strings.TrimSpace(string(input.Provenance))),
		VerificationStatus:     VerificationStatus(strings.TrimSpace(string(input.VerificationStatus))),
		ServingSizeQuantity:    input.ServingSizeQuantity,
		ServingSizeUnit:        strings.TrimSpace(input.ServingSizeUnit),
		YieldFactors:           normalizeYieldFactors(input.YieldFactors),
	}

	if err := validateIngredient(ingredient, requireID); err != nil {
		return Ingredient{}, err
	}

	return ingredient, nil
}

func validateIngredient(ingredient Ingredient, requireID bool) error {
	if requireID && ingredient.ID == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidIngredient)
	}
	if ingredient.LocationID == "" {
		return fmt.Errorf("%w: location id is required", ErrInvalidIngredient)
	}
	if ingredient.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidIngredient)
	}
	if ingredient.Category == "" {
		return fmt.Errorf("%w: category is required", ErrInvalidIngredient)
	}
	if !validUnit(ingredient.BaseUnit) {
		return fmt.Errorf("%w: base unit is invalid", ErrInvalidIngredient)
	}
	if ingredient.CurrentCostPerBaseUnit < 0 {
		return fmt.Errorf("%w: current cost must be non-negative", ErrInvalidIngredient)
	}
	if ingredient.Currency == "" {
		return fmt.Errorf("%w: currency is required", ErrInvalidIngredient)
	}
	if ingredient.OnHandBaseUnits < 0 {
		return fmt.Errorf("%w: on hand quantity must be non-negative", ErrInvalidIngredient)
	}
	if ingredient.ParLevelBaseUnits < 0 {
		return fmt.Errorf("%w: par level must be non-negative", ErrInvalidIngredient)
	}
	if ingredient.Provenance == "" {
		return fmt.Errorf("%w: provenance is required", ErrInvalidIngredient)
	}
	if !validProvenance(ingredient.Provenance) {
		return fmt.Errorf("%w: provenance is invalid", ErrInvalidIngredient)
	}
	if ingredient.VerificationStatus == "" {
		return fmt.Errorf("%w: verification status is required", ErrInvalidIngredient)
	}
	if !validVerificationStatus(ingredient.VerificationStatus) {
		return fmt.Errorf("%w: verification status is invalid", ErrInvalidIngredient)
	}
	if ingredient.ServingSizeQuantity <= 0 {
		return fmt.Errorf("%w: serving size quantity must be positive", ErrInvalidIngredient)
	}
	if ingredient.ServingSizeUnit == "" {
		return fmt.Errorf("%w: serving size unit is required", ErrInvalidIngredient)
	}
	if ingredient.MacrosPerBaseUnit.Calories < 0 ||
		ingredient.MacrosPerBaseUnit.ProteinGrams < 0 ||
		ingredient.MacrosPerBaseUnit.CarbsGrams < 0 ||
		ingredient.MacrosPerBaseUnit.FatGrams < 0 {
		return fmt.Errorf("%w: macros must be non-negative", ErrInvalidIngredient)
	}

	for unit, factor := range ingredient.AlternateUnits {
		if strings.TrimSpace(string(unit)) == "" {
			return fmt.Errorf("%w: alternate unit is required", ErrInvalidIngredient)
		}
		if factor <= 0 {
			return fmt.Errorf("%w: alternate unit factor must be positive", ErrInvalidIngredient)
		}
		if unit == ingredient.BaseUnit {
			return fmt.Errorf("%w: alternate unit cannot match base unit", ErrInvalidIngredient)
		}
	}

	for method, factor := range ingredient.YieldFactors {
		if strings.TrimSpace(method) == "" {
			return fmt.Errorf("%w: yield factor prep method is required", ErrInvalidIngredient)
		}
		if factor <= 0 {
			return fmt.Errorf("%w: yield factor must be positive", ErrInvalidIngredient)
		}
	}

	return nil
}

func normalizeAlternateUnits(input map[Unit]float64) map[Unit]float64 {
	if len(input) == 0 {
		return nil
	}

	keys := make([]string, 0, len(input))
	values := make(map[string]float64, len(input))
	for unit, factor := range input {
		key := strings.TrimSpace(string(unit))
		if key == "" {
			continue
		}
		keys = append(keys, key)
		values[key] = factor
	}
	slices.Sort(keys)

	out := make(map[Unit]float64, len(keys))
	for _, key := range keys {
		out[Unit(key)] = values[key]
	}
	return out
}

func normalizeYieldFactors(input map[string]float64) map[string]float64 {
	if len(input) == 0 {
		return nil
	}

	keys := make([]string, 0, len(input))
	values := make(map[string]float64, len(input))
	for method, factor := range input {
		key := strings.TrimSpace(method)
		if key == "" {
			continue
		}
		keys = append(keys, key)
		values[key] = factor
	}
	slices.Sort(keys)

	out := make(map[string]float64, len(keys))
	for _, key := range keys {
		out[key] = values[key]
	}
	return out
}

func validUnit(unit Unit) bool {
	switch unit {
	case UnitGram, UnitMilliliter, UnitEach:
		return true
	default:
		return false
	}
}

func validProvenance(provenance Provenance) bool {
	switch provenance {
	case ProvenanceManual, ProvenanceUSDA, ProvenanceRestaurantOfficial:
		return true
	default:
		return false
	}
}

func validVerificationStatus(status VerificationStatus) bool {
	switch status {
	case VerificationVerified, VerificationUnverified:
		return true
	default:
		return false
	}
}
