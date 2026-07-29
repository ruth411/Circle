package recipe

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ruth411/circle/internal/core/ingredient"
)

var (
	ErrMenuItemNotFound = errors.New("menu item not found")
	ErrSnapshotNotFound = errors.New("snapshot not found")
	ErrInvalidMenuItem  = errors.New("invalid menu item")
	ErrInvalidSnapshot  = errors.New("invalid snapshot")
)

type CatalogRepository interface {
	GetMenuItem(context.Context, string, string) (MenuItem, error)
	ListMenuItems(context.Context, string) ([]MenuItem, error)
	CreateMenuItem(context.Context, MenuItem) (MenuItem, error)
	UpdateMenuItem(context.Context, MenuItem) (MenuItem, error)
	CreateSnapshot(context.Context, MenuSnapshot) (MenuSnapshot, error)
	GetSnapshot(context.Context, string, string) (MenuSnapshot, error)
	ListSnapshots(context.Context, string) ([]MenuSnapshot, error)
}

type SnapshotResolver interface {
	ResolveRecipe(string) (ResolvedRecipeData, error)
	ResolveModifier(Modifier) (ResolvedModifierData, error)
}

type ResolvedRecipeData struct {
	Macros          ingredient.MacroValues
	IngredientUsage map[string]float64
	IngredientUnits map[string]ingredient.Unit
}

type ResolvedModifierData struct {
	MacroDelta      ingredient.MacroValues
	IngredientUsage map[string]float64
	IngredientUnits map[string]ingredient.Unit
}

type CatalogService struct {
	repo        CatalogRepository
	recipes     Repository
	ingredients IngredientLookup
	resolver    SnapshotResolver
}

type GenerateSnapshotInput struct {
	ID         string
	LocationID string
}

func NewCatalogService(repo CatalogRepository, recipes Repository, ingredients IngredientLookup, resolver SnapshotResolver) *CatalogService {
	return &CatalogService{
		repo:        repo,
		recipes:     recipes,
		ingredients: ingredients,
		resolver:    resolver,
	}
}

func (s *CatalogService) GetMenuItem(ctx context.Context, locationID string, menuItemID string) (MenuItem, error) {
	if strings.TrimSpace(locationID) == "" {
		return MenuItem{}, fmt.Errorf("%w: location id is required", ErrInvalidMenuItem)
	}
	if strings.TrimSpace(menuItemID) == "" {
		return MenuItem{}, fmt.Errorf("%w: menu item id is required", ErrInvalidMenuItem)
	}
	return s.repo.GetMenuItem(ctx, strings.TrimSpace(locationID), strings.TrimSpace(menuItemID))
}

func (s *CatalogService) ListMenuItems(ctx context.Context, locationID string) ([]MenuItem, error) {
	if strings.TrimSpace(locationID) == "" {
		return nil, fmt.Errorf("%w: location id is required", ErrInvalidMenuItem)
	}
	return s.repo.ListMenuItems(ctx, strings.TrimSpace(locationID))
}

func (s *CatalogService) CreateMenuItem(ctx context.Context, input MenuItem) (MenuItem, error) {
	item, err := s.normalizeAndValidateMenuItem(ctx, input)
	if err != nil {
		return MenuItem{}, err
	}
	return s.repo.CreateMenuItem(ctx, item)
}

func (s *CatalogService) UpdateMenuItem(ctx context.Context, input MenuItem) (MenuItem, error) {
	item, err := s.normalizeAndValidateMenuItem(ctx, input)
	if err != nil {
		return MenuItem{}, err
	}
	return s.repo.UpdateMenuItem(ctx, item)
}

func (s *CatalogService) GenerateSnapshot(ctx context.Context, input GenerateSnapshotInput) (MenuSnapshot, error) {
	if s.repo == nil {
		return MenuSnapshot{}, fmt.Errorf("catalog repository is required")
	}
	if s.resolver == nil {
		return MenuSnapshot{}, fmt.Errorf("snapshot resolver is required")
	}

	snapshotID := strings.TrimSpace(input.ID)
	locationID := strings.TrimSpace(input.LocationID)
	if snapshotID == "" {
		return MenuSnapshot{}, fmt.Errorf("%w: snapshot id is required", ErrInvalidSnapshot)
	}
	if locationID == "" {
		return MenuSnapshot{}, fmt.Errorf("%w: location id is required", ErrInvalidSnapshot)
	}

	items, err := s.repo.ListMenuItems(ctx, locationID)
	if err != nil {
		return MenuSnapshot{}, err
	}
	if len(items) == 0 {
		return MenuSnapshot{}, fmt.Errorf("%w: at least one menu item is required to generate a snapshot", ErrInvalidSnapshot)
	}

	snapshotItems := make([]SnapshotItem, len(items))
	for i, item := range items {
		resolvedRecipe, err := s.resolver.ResolveRecipe(item.RecipeID)
		if err != nil {
			return MenuSnapshot{}, err
		}

		groupSnapshots := make([]SnapshotModifierGroup, len(item.ModifierGroups))
		for j, group := range item.ModifierGroups {
			modifierSnapshots := make([]SnapshotModifier, len(group.Modifiers))
			for k, modifier := range group.Modifiers {
				resolvedModifier, err := s.resolver.ResolveModifier(modifier)
				if err != nil {
					return MenuSnapshot{}, err
				}

				modifierSnapshots[k] = SnapshotModifier{
					ModifierID:      modifier.ID,
					Name:            modifier.Name,
					PriceDeltaMinor: modifier.PriceDeltaMinor,
					Currency:        modifier.Currency,
					MacroDelta:      resolvedModifier.MacroDelta,
					IngredientUsage: cloneIngredientUsage(resolvedModifier.IngredientUsage),
					IngredientUnits: cloneIngredientUnits(resolvedModifier.IngredientUnits),
				}
			}

			groupSnapshots[j] = SnapshotModifierGroup{
				GroupID:            group.ID,
				Name:               group.Name,
				SelectionMin:       group.SelectionMin,
				SelectionMax:       group.SelectionMax,
				Required:           group.Required,
				Exclusive:          group.Exclusive,
				DefaultModifierIDs: append([]string(nil), group.DefaultModifierIDs...),
				Modifiers:          modifierSnapshots,
			}
		}

		snapshotItems[i] = SnapshotItem{
			MenuItemID:      item.ID,
			Name:            item.Name,
			Description:     item.Description,
			PriceMinor:      item.PriceMinor,
			Currency:        item.Currency,
			Macros:          resolvedRecipe.Macros,
			IngredientUsage: cloneIngredientUsage(resolvedRecipe.IngredientUsage),
			IngredientUnits: cloneIngredientUnits(resolvedRecipe.IngredientUnits),
			ModifierGroups:  groupSnapshots,
		}
	}

	return s.repo.CreateSnapshot(ctx, MenuSnapshot{
		ID:         snapshotID,
		LocationID: locationID,
		Items:      snapshotItems,
	})
}

func (s *CatalogService) GetSnapshot(ctx context.Context, locationID string, snapshotID string) (MenuSnapshot, error) {
	if strings.TrimSpace(locationID) == "" {
		return MenuSnapshot{}, fmt.Errorf("%w: location id is required", ErrInvalidSnapshot)
	}
	if strings.TrimSpace(snapshotID) == "" {
		return MenuSnapshot{}, fmt.Errorf("%w: snapshot id is required", ErrInvalidSnapshot)
	}
	return s.repo.GetSnapshot(ctx, strings.TrimSpace(locationID), strings.TrimSpace(snapshotID))
}

func (s *CatalogService) ListSnapshots(ctx context.Context, locationID string) ([]MenuSnapshot, error) {
	if strings.TrimSpace(locationID) == "" {
		return nil, fmt.Errorf("%w: location id is required", ErrInvalidSnapshot)
	}
	return s.repo.ListSnapshots(ctx, strings.TrimSpace(locationID))
}

func (s *CatalogService) normalizeAndValidateMenuItem(ctx context.Context, input MenuItem) (MenuItem, error) {
	if s.repo == nil {
		return MenuItem{}, fmt.Errorf("catalog repository is required")
	}
	if s.recipes == nil {
		return MenuItem{}, fmt.Errorf("recipe repository is required")
	}
	if s.ingredients == nil {
		return MenuItem{}, fmt.Errorf("ingredient lookup is required")
	}

	item := MenuItem{
		ID:          strings.TrimSpace(input.ID),
		LocationID:  strings.TrimSpace(input.LocationID),
		RecipeID:    strings.TrimSpace(input.RecipeID),
		Name:        strings.TrimSpace(input.Name),
		Description: strings.TrimSpace(input.Description),
		PriceMinor:  input.PriceMinor,
		Currency:    strings.ToUpper(strings.TrimSpace(input.Currency)),
	}

	if item.ID == "" {
		return MenuItem{}, fmt.Errorf("%w: menu item id is required", ErrInvalidMenuItem)
	}
	if item.LocationID == "" {
		return MenuItem{}, fmt.Errorf("%w: location id is required", ErrInvalidMenuItem)
	}
	if item.RecipeID == "" {
		return MenuItem{}, fmt.Errorf("%w: recipe id is required", ErrInvalidMenuItem)
	}
	if item.Name == "" {
		return MenuItem{}, fmt.Errorf("%w: menu item name is required", ErrInvalidMenuItem)
	}
	if item.PriceMinor < 0 {
		return MenuItem{}, fmt.Errorf("%w: price must be non-negative", ErrInvalidMenuItem)
	}
	if item.Currency == "" {
		return MenuItem{}, fmt.Errorf("%w: currency is required", ErrInvalidMenuItem)
	}
	if _, err := s.recipes.Get(ctx, item.LocationID, item.RecipeID); err != nil {
		if errors.Is(err, ErrRecipeNotFound) {
			return MenuItem{}, fmt.Errorf("%w: recipe %s not found", ErrInvalidMenuItem, item.RecipeID)
		}
		return MenuItem{}, err
	}

	item.ModifierGroups = make([]ModifierGroup, len(input.ModifierGroups))
	seenGroupIDs := map[string]bool{}
	for i, group := range input.ModifierGroups {
		normalized, err := s.normalizeModifierGroup(ctx, item, group)
		if err != nil {
			return MenuItem{}, err
		}
		if seenGroupIDs[normalized.ID] {
			return MenuItem{}, fmt.Errorf("%w: duplicate modifier group id %s", ErrInvalidMenuItem, normalized.ID)
		}
		seenGroupIDs[normalized.ID] = true
		item.ModifierGroups[i] = normalized
	}

	return item, nil
}

func (s *CatalogService) normalizeModifierGroup(ctx context.Context, item MenuItem, input ModifierGroup) (ModifierGroup, error) {
	group := ModifierGroup{
		ID:           strings.TrimSpace(input.ID),
		Name:         strings.TrimSpace(input.Name),
		SelectionMin: input.SelectionMin,
		SelectionMax: input.SelectionMax,
		Required:     input.Required,
		Exclusive:    input.Exclusive,
	}

	if group.ID == "" {
		return ModifierGroup{}, fmt.Errorf("%w: modifier group id is required", ErrInvalidMenuItem)
	}
	if group.Name == "" {
		return ModifierGroup{}, fmt.Errorf("%w: modifier group name is required", ErrInvalidMenuItem)
	}
	if len(input.Modifiers) == 0 {
		return ModifierGroup{}, fmt.Errorf("%w: modifier group %s must include at least one modifier", ErrInvalidMenuItem, group.ID)
	}
	if group.SelectionMin < 0 {
		return ModifierGroup{}, fmt.Errorf("%w: modifier group %s selection min must be non-negative", ErrInvalidMenuItem, group.ID)
	}
	if group.SelectionMax < 0 {
		return ModifierGroup{}, fmt.Errorf("%w: modifier group %s selection max must be non-negative", ErrInvalidMenuItem, group.ID)
	}
	if group.SelectionMax < group.SelectionMin {
		return ModifierGroup{}, fmt.Errorf("%w: modifier group %s selection max must be greater than or equal to selection min", ErrInvalidMenuItem, group.ID)
	}
	if group.Required && group.SelectionMin == 0 {
		return ModifierGroup{}, fmt.Errorf("%w: modifier group %s must require at least one selection", ErrInvalidMenuItem, group.ID)
	}
	if group.Exclusive && group.SelectionMax > 1 {
		return ModifierGroup{}, fmt.Errorf("%w: exclusive modifier group %s cannot allow more than one selection", ErrInvalidMenuItem, group.ID)
	}

	group.Modifiers = make([]Modifier, len(input.Modifiers))
	modifierIDs := map[string]bool{}
	for i, modifier := range input.Modifiers {
		normalized, err := s.normalizeModifier(ctx, item, modifier)
		if err != nil {
			return ModifierGroup{}, err
		}
		if modifierIDs[normalized.ID] {
			return ModifierGroup{}, fmt.Errorf("%w: duplicate modifier id %s", ErrInvalidMenuItem, normalized.ID)
		}
		modifierIDs[normalized.ID] = true
		group.Modifiers[i] = normalized
	}

	group.DefaultModifierIDs = normalizeStringSlice(input.DefaultModifierIDs)
	seenDefaults := map[string]bool{}
	for _, modifierID := range group.DefaultModifierIDs {
		if seenDefaults[modifierID] {
			return ModifierGroup{}, fmt.Errorf("%w: modifier group %s repeats default modifier id %s", ErrInvalidMenuItem, group.ID, modifierID)
		}
		seenDefaults[modifierID] = true
		if !modifierIDs[modifierID] {
			return ModifierGroup{}, fmt.Errorf("%w: modifier group %s default modifier %s is not defined in the group", ErrInvalidMenuItem, group.ID, modifierID)
		}
	}
	if len(group.DefaultModifierIDs) < group.SelectionMin {
		return ModifierGroup{}, fmt.Errorf("%w: modifier group %s default selections are below selection min", ErrInvalidMenuItem, group.ID)
	}
	if group.SelectionMax > 0 && len(group.DefaultModifierIDs) > group.SelectionMax {
		return ModifierGroup{}, fmt.Errorf("%w: modifier group %s default selections exceed selection max", ErrInvalidMenuItem, group.ID)
	}

	return group, nil
}

func (s *CatalogService) normalizeModifier(ctx context.Context, item MenuItem, input Modifier) (Modifier, error) {
	modifier := Modifier{
		ID:              strings.TrimSpace(input.ID),
		Name:            strings.TrimSpace(input.Name),
		PriceDeltaMinor: input.PriceDeltaMinor,
		Currency:        strings.ToUpper(strings.TrimSpace(input.Currency)),
	}

	if modifier.ID == "" {
		return Modifier{}, fmt.Errorf("%w: modifier id is required", ErrInvalidMenuItem)
	}
	if modifier.Name == "" {
		return Modifier{}, fmt.Errorf("%w: modifier %s name is required", ErrInvalidMenuItem, modifier.ID)
	}
	if modifier.Currency == "" {
		return Modifier{}, fmt.Errorf("%w: modifier %s currency is required", ErrInvalidMenuItem, modifier.ID)
	}
	if modifier.Currency != item.Currency {
		return Modifier{}, fmt.Errorf("%w: modifier %s currency must match menu item currency", ErrInvalidMenuItem, modifier.ID)
	}
	if len(input.IngredientDeltas) == 0 {
		return Modifier{}, fmt.Errorf("%w: modifier %s must include at least one ingredient delta", ErrInvalidMenuItem, modifier.ID)
	}

	modifier.IngredientDeltas = make([]IngredientDelta, len(input.IngredientDeltas))
	seenIngredients := map[string]bool{}
	for i, delta := range input.IngredientDeltas {
		normalized, err := s.normalizeIngredientDelta(ctx, item.LocationID, modifier.ID, delta)
		if err != nil {
			return Modifier{}, err
		}
		key := normalized.IngredientID + "|" + string(normalized.Unit) + "|" + normalized.PrepMethod
		if seenIngredients[key] {
			return Modifier{}, fmt.Errorf("%w: modifier %s repeats ingredient delta for %s", ErrInvalidMenuItem, modifier.ID, normalized.IngredientID)
		}
		seenIngredients[key] = true
		modifier.IngredientDeltas[i] = normalized
	}

	return modifier, nil
}

func (s *CatalogService) normalizeIngredientDelta(ctx context.Context, locationID string, modifierID string, input IngredientDelta) (IngredientDelta, error) {
	delta := IngredientDelta{
		IngredientID: strings.TrimSpace(input.IngredientID),
		Quantity:     input.Quantity,
		Unit:         ingredient.Unit(strings.TrimSpace(string(input.Unit))),
		PrepMethod:   strings.TrimSpace(input.PrepMethod),
	}

	if delta.IngredientID == "" {
		return IngredientDelta{}, fmt.Errorf("%w: modifier %s ingredient id is required", ErrInvalidMenuItem, modifierID)
	}
	if delta.Quantity == 0 {
		return IngredientDelta{}, fmt.Errorf("%w: modifier %s ingredient %s quantity must be non-zero", ErrInvalidMenuItem, modifierID, delta.IngredientID)
	}
	ing, err := s.ingredients.Get(ctx, locationID, delta.IngredientID)
	if err != nil {
		return IngredientDelta{}, fmt.Errorf("%w: ingredient %s not found", ErrInvalidMenuItem, delta.IngredientID)
	}
	if _, err := ing.ToBaseUnit(absFloat(delta.Quantity), delta.Unit); err != nil {
		return IngredientDelta{}, fmt.Errorf("%w: %v", ErrInvalidMenuItem, err)
	}

	return delta, nil
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneIngredientUsage(values map[string]float64) map[string]float64 {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]float64, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloneIngredientUnits(values map[string]ingredient.Unit) map[string]ingredient.Unit {
	if len(values) == 0 {
		return nil
	}

	out := make(map[string]ingredient.Unit, len(values))
	for ingredientID, unit := range values {
		out[ingredientID] = unit
	}
	return out
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
