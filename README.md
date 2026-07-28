•⁠  ⁠# Circle

  Circle is a restaurant operating system with *macro nutrition built into the core data model*.

  Most restaurant systems split the business into separate tools:
  - front of house for orders and payments
  - back of house for inventory, purchasing, and reporting

  Circle is built on a different idea:

  *the ingredient is the source of truth*

  One ingredient record carries:
  - units
  - cost
  - stock
  - macro nutrition

  From there:
  - recipes are built from ingredients
  - menu items are built from recipes
  - modifiers change price, stock, and macros together
  - orders resolve real item-level macros at the time of sale
  - diners can scan a receipt token and claim only the items they personally ate

  This makes nutrition part of the transaction itself, not a rough estimate added later.

  ---

  ## Why this project exists

  People who track food usually cannot log customized restaurant meals correctly.

  If someone orders a build-your-own bowl, sandwich, or salad, the nutrition number they need usually does not exist in a
  usable form. They guess.

  Circle solves that by letting a restaurant produce *order-specific macro totals* from the same data it already needs to
  run operations:
  - ingredients
  - recipes
  - modifiers
  - order lines

  The goal is not perfect laboratory accuracy.
  The goal is a much better, honest, and defensible estimate than guesswork.

  ---

  ## Accuracy stance

  Circle treats nutrition as an *estimate*, not an exact fact.

  Target accuracy is about *90%*, and that is intentional.

  Why not 100%?
  - portions vary
  - prep varies by staff
  - ingredient sourcing changes
  - real kitchens are not laboratory environments

  Every nutrition surface in Circle should clearly communicate that macro values are estimated.

  ---

  ## Core product idea

  The system works like this:

  1. Ingredients are entered with units, cost, stock, and macros per base unit.
  2. Recipes are built from those ingredients.
  3. Menu items are built from recipes.
  4. Modifiers add or remove real ingredient quantities.
  5. Orders resolve final prices and macro totals at the line level.
  6. When the order is closed, the nutrition values are frozen.
  7. A receipt token can be scanned by diners.
  8. Each diner can select only the items they ate and get their own macro totals.

  That receipt-token claim flow is one of Circle’s defining features.

  ---

  ## What Circle is trying to become

  Circle is being built as a *full restaurant operating system*, not just a nutrition calculator.

  Planned system areas include:
  - ingredient master
  - recipe and menu management
  - nutrition engine
  - ordering and checks
  - inventory depletion
  - purchasing
  - accounting
  - labor
  - diner receipt scan and claim flow
  - reporting

  The long-term goal is one coherent system where:
  - price
  - cost
  - stock
  - and macros

  all come from the same underlying ingredient graph.

  ---

  ## Current project status

  This is a *personal flagship prototype*.

  Priorities:
  - correctness of the domain model
  - strong invariants
  - boring, readable code
  - phase-by-phase progress
  - security review with Snyk before phase commits

  This project is being built for coherence first, not feature breadth first.

  ---

  ## Guiding principles

  ### 1. Ingredient-first design
  The ingredient is the root object of the system.

  ### 2. Macros are native
  Nutrition is computed from the same graph as cost and inventory.

  ### 3. Served nutrition is immutable
  Once an order is closed, its macro values never change retroactively.

  ### 4. Boring code over clever code
  The project favors:
  - plain structs
  - direct service methods
  - explicit validation
  - normal SQL
  - forward-only migrations
  - strict boundaries

  ### 5. Thin clients
  Business logic stays server-side.
  Clients capture and render; they should not own pricing or macro rules.

  ### 6. Hard module boundaries
  This is a modular monolith, not a free-for-all codebase.

  ---

  ## Architecture

  Circle is designed as a *modular monolith with hard internal boundaries*.

  ### Shared core
  - ⁠ ingredient ⁠
  - ⁠ recipe ⁠
  - ⁠ nutrition ⁠

  ### Domain modules
  - ⁠ ordering ⁠
  - ⁠ inventory ⁠
  - ⁠ purchasing ⁠
  - ⁠ accounting ⁠
  - ⁠ labor ⁠

  ### Support and edge modules
  - ⁠ identity ⁠
  - ⁠ tenancy ⁠
  - ⁠ diner ⁠
  - ⁠ reporting ⁠

  ### Platform
  - database
  - outbox/events
  - HTTP API
  - config
  - logging

  Key rules:
  - modules can depend on shared core
  - modules must not import each other’s internals
  - no module may read or write another module’s tables
  - cross-module communication must happen through interfaces or events

  ---

  ## Important domain rules

  These are non-negotiable:

  - macros are only hand-entered at the ingredient level
  - recipe, menu item, and modifier macros are always derived
  - money is stored in integer minor units
  - quantities are stored in base units
  - every domain row is location-scoped
  - inventory movements are append-only
  - ledger postings are append-only
  - order capture must be idempotent
  - modifiers must affect price, stock, and macros together
  - nutrition must always be presented as an estimate

  ---

  ## What makes modifiers important

  In many restaurant systems, modifiers only affect price.

  In Circle, a modifier must carry:
  - a price delta
  - an ingredient delta
  - a derived macro delta
  - a derived inventory delta

  Examples:
  - extra chicken
  - no cheese
  - sub brown rice for white rice

  If modifiers are modeled as price-only, Circle’s nutrition and inventory model breaks.

  ---

  ## Menu snapshots

  Order capture clients should not read live mutable menu data directly.

  Circle uses *immutable menu snapshots* that contain:
  - menu items
  - modifiers
  - prices
  - precomputed macro deltas

  This gives:
  - deterministic order capture
  - safer offline behavior
  - historical correctness

  Every order records the snapshot version it used.

  ---

  ## Diner scan flow

  When an order is completed, Circle issues a *receipt token*.

  That token:
  - is opaque
  - is not guessable
  - is not derived from the order ID
  - only exposes itemized macro data

  A diner can:
  - scan the token
  - select the items they ate
  - view their macro totals
  - export or copy that nutrition information

  Important constraints:
  - claims are anonymous
  - multiple people can claim from the same order
  - no payment or staff data should be exposed through the token

  ---

  ## Non-goals

  These are deliberately out of scope for now:

  - real card-present payment processing
  - payroll and tax filing
  - printer and hardware integration
  - audited GAAP statement formatting
  - direct third-party nutrition tracker integrations
  - multi-region infrastructure
  - micronutrient tracking

  Where needed, these should be stubbed cleanly rather than overbuilt.

  ---

  ## Suggested repository shape

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

  ———

  ## Build philosophy

  Circle is being built in vertical slices, not giant isolated modules.

  The preferred order is:

  1. core ingredient and recipe spine
  2. first end-to-end slice from ingredient to order to depletion
  3. snapshot and offline order capture
  4. diner scan and claims
  5. purchasing and cost
  6. accounting
  7. labor
  8. deeper reporting

  This helps catch model mistakes early.

  ———

  ## Security and quality workflow

  For each development phase, the expected workflow is:

  1. implement the phase
  2. run Snyk scans
  3. fix reported issues
  4. run Snyk again
  5. repeat until the phase reaches zero reported bugs
  6. only then commit the phase

  Security scanning is part of the phase exit criteria, not an optional cleanup step.

  ———

  ## Vision

  Circle is not trying to be “just another restaurant app.”

  It is trying to make this statement true:

  > The same ingredient data that runs the restaurant should also power the diner’s nutrition result.

  If that model is correct, everything else becomes simpler, more honest, and more connected.

  ———

  ## Notes

  - Nutrition values are estimates.
  - Historical served nutrition must never be rewritten.
  - The ingredient graph is the foundation of the whole system.
  - This project prefers coherence over speed and correctness over feature count.
