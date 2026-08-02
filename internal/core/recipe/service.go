package recipe

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ruth411/circle/internal/core/ingredient"
)

var (
	ErrRecipeNotFound      = errors.New("recipe not found")
	ErrRecipeAlreadyExists = errors.New("recipe already exists")
	ErrInvalidRecipe       = errors.New("invalid recipe")
	DefaultMaxDepth        = 5
)

type Repository interface {
	Get(context.Context, string, string) (Recipe, error)
	List(context.Context, string) ([]Recipe, error)
	Create(context.Context, Recipe) (Recipe, error)
	Update(context.Context, Recipe) (Recipe, error)
}

type IngredientLookup interface {
	Get(context.Context, string, string) (ingredient.Ingredient, error)
}

type Service struct {
	repo        Repository
	ingredients IngredientLookup
	maxDepth    int
}

type UpsertInput struct {
	ID         string
	LocationID string
	Name       string
	YieldCount float64
	Lines      []RecipeLine
}

func NewService(repo Repository, ingredients IngredientLookup) *Service {
	return &Service{
		repo:        repo,
		ingredients: ingredients,
		maxDepth:    DefaultMaxDepth,
	}
}

func (s *Service) Get(ctx context.Context, locationID string, recipeID string) (Recipe, error) {
	if strings.TrimSpace(locationID) == "" {
		return Recipe{}, fmt.Errorf("%w: location id is required", ErrInvalidRecipe)
	}
	if strings.TrimSpace(recipeID) == "" {
		return Recipe{}, fmt.Errorf("%w: recipe id is required", ErrInvalidRecipe)
	}
	return s.repo.Get(ctx, strings.TrimSpace(locationID), strings.TrimSpace(recipeID))
}

func (s *Service) List(ctx context.Context, locationID string) ([]Recipe, error) {
	if strings.TrimSpace(locationID) == "" {
		return nil, fmt.Errorf("%w: location id is required", ErrInvalidRecipe)
	}
	return s.repo.List(ctx, strings.TrimSpace(locationID))
}

func (s *Service) Create(ctx context.Context, input UpsertInput) (Recipe, error) {
	recipe, err := s.normalizeAndValidate(ctx, input)
	if err != nil {
		return Recipe{}, err
	}
	return s.repo.Create(ctx, recipe)
}

func (s *Service) Update(ctx context.Context, input UpsertInput) (Recipe, error) {
	recipe, err := s.normalizeAndValidate(ctx, input)
	if err != nil {
		return Recipe{}, err
	}
	return s.repo.Update(ctx, recipe)
}

func (s *Service) normalizeAndValidate(ctx context.Context, input UpsertInput) (Recipe, error) {
	if s.repo == nil {
		return Recipe{}, fmt.Errorf("recipe repository is required")
	}
	if s.ingredients == nil {
		return Recipe{}, fmt.Errorf("ingredient lookup is required")
	}

	out := Recipe{
		ID:         strings.TrimSpace(input.ID),
		LocationID: strings.TrimSpace(input.LocationID),
		Name:       strings.TrimSpace(input.Name),
		YieldCount: input.YieldCount,
		Lines:      make([]RecipeLine, len(input.Lines)),
	}

	if out.ID == "" {
		return Recipe{}, fmt.Errorf("%w: recipe id is required", ErrInvalidRecipe)
	}
	if out.LocationID == "" {
		return Recipe{}, fmt.Errorf("%w: location id is required", ErrInvalidRecipe)
	}
	if out.Name == "" {
		return Recipe{}, fmt.Errorf("%w: recipe name is required", ErrInvalidRecipe)
	}
	if out.YieldCount <= 0 {
		return Recipe{}, fmt.Errorf("%w: yield count must be positive", ErrInvalidRecipe)
	}
	if len(input.Lines) == 0 {
		return Recipe{}, fmt.Errorf("%w: at least one recipe line is required", ErrInvalidRecipe)
	}

	for i, line := range input.Lines {
		out.Lines[i] = RecipeLine{
			LineNumber: i + 1,
			TargetType: LineTargetType(strings.TrimSpace(string(line.TargetType))),
			TargetID:   strings.TrimSpace(line.TargetID),
			Quantity:   line.Quantity,
			Unit:       ingredient.Unit(strings.TrimSpace(string(line.Unit))),
			PrepMethod: strings.TrimSpace(line.PrepMethod),
		}
	}

	if err := s.validateTargets(ctx, out); err != nil {
		return Recipe{}, err
	}

	return out, nil
}

func (s *Service) validateTargets(ctx context.Context, root Recipe) error {
	graph := map[string]Recipe{
		root.ID: root,
	}

	for _, line := range root.Lines {
		switch line.TargetType {
		case LineTargetIngredient:
			ing, err := s.ingredients.Get(ctx, root.LocationID, line.TargetID)
			if err != nil {
				if errors.Is(err, ingredient.ErrNotFound) {
					return fmt.Errorf("%w: ingredient %s not found", ErrInvalidRecipe, line.TargetID)
				}
				return err
			}
			if _, err := ing.ToBaseUnit(line.Quantity, line.Unit); err != nil {
				return fmt.Errorf("%w: %v", ErrInvalidRecipe, err)
			}
		case LineTargetRecipe:
			if line.TargetID == root.ID {
				return fmt.Errorf("%w: recipe %s cannot reference itself", ErrInvalidRecipe, root.ID)
			}
			if line.Unit != ingredient.UnitEach {
				return fmt.Errorf("%w: nested recipe %s must use unit %s", ErrInvalidRecipe, line.TargetID, ingredient.UnitEach)
			}
			if err := s.collectRecipeGraph(ctx, root.LocationID, line.TargetID, graph); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: unknown target type %q", ErrInvalidRecipe, line.TargetType)
		}
	}

	if err := ValidateRecipeGraph(root.ID, graph, s.depthLimit()); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRecipe, err)
	}

	return nil
}

func (s *Service) collectRecipeGraph(ctx context.Context, locationID string, recipeID string, graph map[string]Recipe) error {
	if _, ok := graph[recipeID]; ok {
		return nil
	}

	child, err := s.repo.Get(ctx, locationID, recipeID)
	if err != nil {
		if errors.Is(err, ErrRecipeNotFound) {
			return fmt.Errorf("%w: recipe %s not found", ErrInvalidRecipe, recipeID)
		}
		return err
	}
	graph[recipeID] = child

	for _, line := range child.Lines {
		if line.TargetType == LineTargetRecipe {
			if err := s.collectRecipeGraph(ctx, locationID, line.TargetID, graph); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Service) depthLimit() int {
	if s.maxDepth > 0 {
		return s.maxDepth
	}
	return DefaultMaxDepth
}
