# Circle

Circle is a restaurant system built around one idea:

Restaurants already know what goes into an order. Circle uses that same data to
estimate the calories, protein, carbs, and fat for the exact meal a customer
ordered.

## What it does

- Stores ingredients with units, cost, stock, and macros.
- Builds recipes and menu items from those ingredients.
- Tracks modifiers like "extra chicken" or "no cheese" as changes to price,
  inventory, and macros.
- Saves the nutrition totals for each order at the time of sale.
- Gives the customer a receipt QR code so they can see and claim the items they
  ate and get their own macro totals.

## The problem

People who track food usually have to guess when they eat at restaurants,
especially for customizable meals like bowls, salads, or sandwiches.

Circle tries to make that guess much better by connecting restaurant order data
to simple, usable macro estimates.

## Important limitation

Circle does not promise exact nutrition numbers.

Restaurant food changes based on portion size, prep style, and ingredient
source. The goal is a useful estimate, roughly 90% accurate, not a perfect lab
result. Any nutrition value shown by Circle should be treated as an estimate.

## What Circle is not trying to do

This project is intentionally not building:

- real card-present payment processing
- payroll or tax filing
- printer or hardware integrations
- audited financial statement output
- direct integrations with nutrition apps right now
- multi-region or high-availability infrastructure
- micronutrient tracking

## How the system is shaped

Circle is planned as a modular monolith:

- one deployable app
- separate domain modules with hard boundaries
- one shared core for ingredients, recipes, and nutrition
- Postgres as the main database

Main domain areas:

- `ingredient`: source of truth for cost, stock, units, and macros
- `recipe`: recipes, menu items, modifiers, menu snapshots
- `nutrition`: macro calculations
- `ordering`: orders, checks, tenders, receipt tokens
- `inventory`: stock movements and depletion
- `purchasing`: vendors, purchase orders, invoices
- `accounting`: double-entry ledger
- `labor`: shifts and labor cost
- `diner`: public receipt-token flow for macro claims

## Current status

This is a personal prototype, built solo, with no production users.

The priority is getting the domain model right:

- ingredient data
- recipe and modifier structure
- nutrition roll-up
- inventory and accounting links

## Suggested build order

1. Build the ingredient, recipe, modifier, and nutrition core.
2. Ship one end-to-end flow from ingredient to order to receipt token.
3. Add menu snapshots and offline order capture.
4. Add the diner claim flow.
5. Add purchasing, accounting, labor, and deeper reporting after that.

## Repo direction

The intended structure is:

```text
/cmd/circle
/internal/core/ingredient
/internal/core/recipe
/internal/core/nutrition
/internal/ordering
/internal/inventory
/internal/purchasing
/internal/accounting
/internal/labor
/internal/identity
/internal/tenancy
/internal/diner
/internal/reporting
/internal/platform
/migrations
/clients/pos
/clients/kds
/clients/backoffice
/clients/scan
/docs
```
