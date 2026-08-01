# Phase A3 Snapshot Generation Design

## Goal

Make menu snapshots user-reachable so a restaurant can define ingredients,
recipes, and menu items, then generate a frozen sellable menu snapshot that
ordering can use without relying on seed data.

## Product Rules

- Ingredient macros are the source of truth.
- Recipe macros are derived from ingredient data, not stored as independent
  truth.
- Weight-based ingredients use macros normalized to one gram.
- Ingredients whose natural base unit is not grams keep their natural base
  unit, such as `each`.
- A menu item snapshot stores nutrition for one sellable serving, not the
  full prep batch.
- Modifier snapshots store only the delta caused by that modifier: add,
  remove, or change.
- Final order-line macros equal base item macros plus or minus the selected
  modifier deltas.
- Once created, a snapshot is frozen forever. Later recipe edits produce new
  snapshots and do not rewrite old orders.

## Current State

- `recipe.CatalogService.GenerateSnapshot` already exists and already knows
  how to build `recipe.MenuSnapshot` objects.
- `recipe.CatalogService` expects a `recipe.SnapshotResolver`.
- `nutrition.Calculator` already knows how to resolve recipe macros,
  modifier macro deltas, ingredient usage, and units.
- The missing piece is a thin boundary adapter plus HTTP routes that expose
  snapshot creation and retrieval.

## Recommended Architecture

Use a thin adapter between catalog and nutrition, keep the business rules in
`recipe.CatalogService`, and add small HTTP routes that only decode requests,
authorize access, and call the service.

Flow:

`HTTP -> CatalogService -> SnapshotResolverAdapter -> NutritionCalculator -> CatalogRepository`

Responsibilities:

- HTTP layer:
  - `POST /menu-snapshots`
  - `GET /menu-snapshots`
  - `GET /menu-snapshots/{id}`
  - Performs auth, location scoping, strict JSON decoding, and error mapping.
- Catalog service:
  - Owns the business action "generate a frozen snapshot for this restaurant".
  - Loads menu items for the restaurant.
  - Resolves base item nutrition and modifier deltas.
  - Builds and persists the immutable snapshot.
- Snapshot resolver adapter:
  - Translates nutrition results into `recipe.SnapshotResolver`.
  - Maps recipe resolution to per-serving item macros.
  - Maps modifier resolution to delta macros.
- Nutrition calculator:
  - Remains the single place that performs nutrition math.
- `main.go`:
  - Wires the calculator, adapter, catalog service, and HTTP dependencies.

## Adapter Contract

The adapter should live outside the `recipe` package to avoid an import cycle.
The smallest clean place is a thin boundary package such as
`internal/platform/resolve`.

Suggested shape:

```go
type SnapshotResolver struct {
    Calculator nutrition.Calculator
}

func (r SnapshotResolver) ResolveRecipe(recipeID string) (recipe.ResolvedRecipeData, error) {
    resolved, err := r.Calculator.ResolveRecipe(recipeID)
    if err != nil {
        return recipe.ResolvedRecipeData{}, err
    }
    return recipe.ResolvedRecipeData{
        Macros:          resolved.PerServing,
        IngredientUsage: cloneUsage(resolved.IngredientUsage),
        IngredientUnits: cloneUnits(resolved.IngredientUnits),
    }, nil
}

func (r SnapshotResolver) ResolveModifier(modifier recipe.Modifier) (recipe.ResolvedModifierData, error) {
    resolved, err := r.Calculator.ResolveModifier(modifier)
    if err != nil {
        return recipe.ResolvedModifierData{}, err
    }
    return recipe.ResolvedModifierData{
        MacroDelta:      resolved.MacroDelta,
        IngredientUsage: cloneUsage(resolved.IngredientUsage),
        IngredientUnits: cloneUnits(resolved.IngredientUnits),
    }, nil
}
```

Important mapping rule:

- Use `ResolvedRecipe.PerServing`, not `ResolvedRecipe.TotalMacros`.

That matches the product rule that a menu item snapshot represents one item as
sold to one customer.

## HTTP Contract

### POST /menu-snapshots

Request body:

```json
{
  "id": "snap-2026-08-01-menu-v1"
}
```

Behavior:

- Reads `location_id` from the authenticated request context.
- Calls `CatalogService.GenerateSnapshot`.
- Returns the created snapshot.

Errors:

- `400 invalid_snapshot` for bad input.
- `404` only if the underlying service eventually exposes a scoped not-found
  condition worth surfacing directly.
- `500 internal_error` for unexpected failures.

### GET /menu-snapshots

Behavior:

- Returns all snapshots for the current location.
- Response can be the existing repository shape; no new summary abstraction is
  needed yet.

### GET /menu-snapshots/{id}

Behavior:

- Returns the full immutable snapshot for the current location, including
  snapshot items, modifier groups, and modifier deltas.

## Data Semantics

Snapshot item:

- Name, description, price, and currency come from the current menu item.
- Macros come from resolved recipe per-serving nutrition.
- Ingredient usage and units come from resolved recipe usage.

Snapshot modifier:

- Name, price delta, and currency come from the current modifier definition.
- Macro delta comes from resolved modifier nutrition.
- Ingredient usage and units come from resolved modifier usage.

Historical behavior:

- If a restaurant changes a recipe after snapshot version 3 exists, version 3
  remains unchanged.
- A new snapshot version reflects the changed recipe.
- Orders keep referencing the snapshot version they were created against.

## Files

- Create: `internal/platform/resolve/snapshot_resolver.go`
- Create: `internal/platform/resolve/snapshot_resolver_test.go`
- Create: `internal/platform/httpapi/snapshots.go`
- Create: `internal/platform/httpapi/snapshots_test.go`
- Modify: `internal/platform/httpapi/server.go`
- Modify: `cmd/circle/main.go`

No schema change is required for A3 because snapshot tables already exist.

## Error Handling

- Resolver errors bubble up through `CatalogService.GenerateSnapshot`.
- Missing ingredient or recipe resolution should fail snapshot generation; do
  not create partial snapshots.
- Empty menu item sets should continue failing as invalid snapshots.
- Keep route decoding strict:
  - unknown JSON fields rejected
  - single-object body required
  - request size capped at 1 MiB

## Testing Strategy

Adapter tests:

- recipe resolution maps `PerServing`, not `TotalMacros`
- modifier resolution preserves positive and negative deltas
- ingredient usage and units are copied through

HTTP tests:

- `POST /menu-snapshots` creates a snapshot through the service
- `GET /menu-snapshots` returns location-scoped snapshots
- `GET /menu-snapshots/{id}` returns one snapshot
- strict JSON handling and error mapping match the rest of the API

Integration proof for A3:

- Generate a snapshot through the service or API.
- Create an order against that generated snapshot ID.
- Verify ordering succeeds without depending on seeded snapshot IDs.

## Non-Goals

- No new snapshot-editing route. Snapshots are immutable.
- No new admin workflow for auto-version naming. Caller provides the snapshot
  ID for now.
- No refactor of nutrition math. Reuse the current calculator.
- No read-model expansion beyond the three snapshot routes.

## Why This Design

This is the smallest change that makes the product differentiator real.

- It reuses the existing catalog service and existing nutrition calculator.
- It keeps business rules out of `main.go` and out of HTTP handlers.
- It avoids duplicate nutrition logic.
- It preserves historical truth for old orders.
