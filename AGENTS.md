# AGENTS.md — Circle

> Context file for AI coding agents working in this repository.
> Read this fully before making changes. It defines what Circle is, how it is
> structured, and which rules are non-negotiable.

---

## 1. What Circle is

Circle is a **restaurant operating system with macro nutrition native to the data model.**

Most restaurant software splits into two ecosystems: a front-of-house POS
(order entry, payments, kitchen) and a back-office platform (inventory,
purchasing, accounting, labor). They are separate products because the
**ingredient** — the object that carries cost, stock, and nutrition — is split
across them. Neither side owns it completely.

Circle unifies that object. One ingredient record carries its units, its cost,
its stock, and its macros. Recipes are composed from ingredients. Menu items
are recipes. Orders are menu items plus modifiers. Because everything is built
from the same graph, nutrition is not a report generated after the fact — it is
computed the same way price is computed, at the same moment, from the same data.

The visible payoff for the diner: every order produces a **single scannable
token** (QR on the printed or emailed receipt). Anyone at the table scans the
same token, selects the items they personally ate, and gets their own macro
totals to carry into their nutrition tracker.

### The problem being solved

People who track their food cannot accurately log customized restaurant meals.
A build-your-own bowl or sandwich has no clean number to enter, so they guess.
Restaurant order data and personal nutrition trackers have never been connected.
Circle makes the restaurant emit accurate, order-specific macro data in a form
the diner can actually use.

### Accuracy stance

**Target accuracy is ~90%, not 100%, and this is deliberate.** Real
away-from-home food varies by portion, prep, and ingredient sourcing. Exact
numbers are not achievable and must never be promised. Calculated values from a
verified ingredient database are honest, defensible, and materially better than
the guesswork diners do today.

Every surface that displays nutrition must carry an estimate disclaimer.
Never render a macro value as an exact fact.

### Project status

Personal project. Full prototype of the complete ecosystem, built solo.
No production users. Optimize for **coherence and correctness of the domain
model** over feature breadth or scale.

---

## 2. Non-goals and deliberate stubs

Do not build these. If a task seems to require one, stub it behind an interface
and note the assumption.

| Area | Why it is out of scope | What to do instead |
|---|---|---|
| Card-present payment processing | PCI scope, EMV certification, hardware. Compliance work, not engineering. | `PaymentProvider` interface with a `MockProvider` that always succeeds/fails on command. |
| Payroll and tax filing | Jurisdictional rule-chasing, no learning value, endless maintenance. | Record labor hours and cost only. No tax computation. |
| Physical hardware and thermal printers | Not buildable solo, not interesting. | Render receipts as documents (HTML/PDF). Printing is a driver detail. |
| GAAP-compliant financial statements | Accounting *is* in scope; regulatory conformance is not. | Build a real double-entry GL. Do not chase audited statement formats. |
| Direct integrations with third-party nutrition trackers | Their write APIs are largely closed. | Export macros as copyable text and structured payload. Apple Health / Google Fit is the eventual first real integration. |
| Multi-region, high-availability infrastructure | Prototype scale. | Single Postgres, single deployment. |

**Micronutrients are not in scope.** Circle tracks macros: calories, protein,
carbohydrates, fat. The schema should not make micronutrients impossible to add
later, but do not build for them now.

---

## 3. Domain glossary

Use these terms exactly. Consistency here matters more than elegance.

- **Ingredient** — an atomic purchasable/stockable food component (e.g. grilled
  chicken thigh, romaine, olive oil). Carries units, cost, stock, and macros per
  base unit. The root object of the entire system.
- **Base unit** — the canonical unit an ingredient's macros and cost are stored
  against (usually grams or milliliters). All other units convert to it.
- **Yield factor** — the ratio of cooked/prepped weight to raw weight. Applied
  when a recipe uses a prepped form of a raw ingredient.
- **Recipe** — a composition of ingredients (and optionally other recipes) with
  quantities. Has a computed cost and computed macros. Not necessarily sellable.
- **Menu item** — a sellable recipe with a price, a name, and menu placement.
- **Modifier** — a customization applied to a menu item at order time. Carries a
  **price delta**, an **ingredient delta**, and therefore a **macro delta** and
  an **inventory delta**. "Extra guac", "no cheese", "sub brown rice".
- **Modifier group** — a set of modifiers with selection rules (min, max,
  required, exclusive).
- **Menu snapshot** — an immutable, versioned, denormalized projection of the
  full menu (items + modifiers + prices + macro deltas) pushed to order-capture
  clients. What terminals actually read.
- **Order** — a customer's set of selected menu items with modifiers.
- **Check** — the billable container for an order. Can be split.
- **Tender** — a payment applied to a check.
- **Depletion** — the inventory movement caused by an order being fulfilled.
- **Receipt token** — the opaque identifier encoded in the receipt QR that
  resolves to an order's itemized macro breakdown.
- **Claim** — one diner's selection of which items from an order they personally
  ate, and the macro totals derived from it.

---

## 4. Architecture

### 4.1 Shape

**Modular monolith with hard internal boundaries.** One deployable binary,
multiple domain modules with enforced separation.

This is a deliberate choice, not a shortcut. A single engineer does not benefit
from service discovery, network failure modes, distributed transactions, and N
deployment pipelines. Ordering and inventory in particular need transactional
consistency, which is painful across a network and free in-process.

Because the boundaries are real, extracting any module into its own service
later is mechanical rather than a rewrite. **The boundaries must therefore be
respected as strictly as if they were network calls.**

### 4.2 Layers

```
Clients          POS terminal · Kitchen display · Back office · Diner scan
                                      |
Edge             API gateway + identity/authn/authz + tenant resolution
                                      |
Domain modules   Ordering · Inventory · Purchasing · Accounting · Labor
                                      |
Shared core      Ingredient master · Recipe & menu · Nutrition engine
                                      |
Storage          Transactional DB (Postgres) · Reporting store · Object storage
```

### 4.3 Dependency rules — enforced

1. **Modules may depend on the shared core. The shared core depends on nothing
   above it.**
2. **Modules must not import each other's internal packages.** Cross-module
   communication happens through published interfaces or domain events only.
3. **No module reads or writes another module's tables. Ever.** No joins across
   module boundaries in SQL. If you need another module's data, call its
   interface or subscribe to its events.
4. **Clients contain no business logic.** Pricing, macro math, and validation
   live server-side. Clients render and capture.
5. Dependencies point downward. A lower layer never imports an upper layer.

If a task appears to require breaking one of these rules, the model is wrong —
stop and surface the problem rather than working around it.

### 4.4 Module catalog

**Shared core**

- `ingredient` — ingredient master, units and conversions, yield factors,
  cost basis, macro values per base unit, supplier mapping.
- `recipe` — recipe composition, nested recipes, menu items, modifier groups
  and modifiers, menu structure, menu snapshot generation and versioning.
- `nutrition` — macro roll-up over the recipe graph, yield/retention handling,
  modifier delta computation, per-order macro resolution, confidence flags.

**Domain modules**

- `ordering` — order lifecycle, check management, splits, tenders (via the
  payment interface), voids/comps, kitchen routing, receipt token issuance.
- `inventory` — stock levels, depletion from orders, receiving from purchase
  orders, counts, waste, variance (theoretical vs actual usage).
- `purchasing` — vendors, purchase orders, vendor invoices, AP, cost updates
  flowing back to the ingredient master.
- `accounting` — double-entry general ledger, chart of accounts, journal
  entries posted from sales, depletion, purchases, and labor.
- `labor` — employees, roles, shifts, scheduling, clock in/out, labor cost.

**Edge / support**

- `identity` — users, roles, permissions, staff PIN auth, session management.
- `tenancy` — organization, restaurant, location hierarchy. Every domain row is
  scoped to a location.
- `diner` — public-facing token resolution, claim selection, macro export.
  This is the only module serving unauthenticated public traffic; treat it as
  hostile-input territory.
- `reporting` — read models and aggregates. Consumes events, owns no source of
  truth.

### 4.5 Deployment topology

One binary. One Postgres database, one schema per module (schemas are how the
"no cross-module tables" rule is made visible and enforceable). An outbox table
for domain events. The reporting store may start as materialized views in the
same database and split out only if it becomes a bottleneck.

---

## 5. The data model

This is the part that must be right. Everything else is replaceable.

### 5.1 Ingredient

The root object. One record serves nutrition, costing, and inventory
simultaneously — that unification is Circle's entire architectural thesis.

Conceptually carries:

- identity: id, location scope, name, category
- units: base unit, list of alternate units with conversion factors to base
- nutrition: calories, protein, carbs, fat **per base unit**
- yield: yield factors per prep method (raw → cooked)
- cost: current cost per base unit, cost method (last, average)
- stock: on-hand quantity in base units, par level
- provenance: data source for the macro values, verification status

**Rules**

- Macros and cost are always stored per base unit. Never per "serving",
  never per purchase unit. All display conversions happen at read time.
- Every ingredient must have a macro provenance and verification status.
  Unverified ingredients are usable but must be surfaced as low-confidence
  everywhere downstream.
- Changing an ingredient's macro values does **not** retroactively change any
  order that has already been served. See §6.

### 5.2 Recipe and menu item

- A recipe has line items, each referencing an ingredient or another recipe,
  with a quantity and unit.
- Nesting is allowed but bounded — enforce a maximum depth and detect cycles at
  write time, not at read time.
- Cost and macros roll up through the graph. Roll-up is a pure function of the
  graph; it is never hand-entered.
- A menu item is a recipe plus price, display name, description, and menu
  placement.

### 5.3 Modifier — the critical object

**A modifier is one guest action that must move three numbers: price, macros,
and stock.** This is where most POS data models fail, because they only model
price. If modifiers are price-only in this system, Circle does not work and
retrofitting is expensive.

A modifier therefore carries:

- price delta
- an **ingredient delta list**: ingredient references with signed quantities
  (positive for additions, negative for removals, or a substitution pair)
- derived macro delta (computed from the ingredient delta — never hand-entered)
- derived inventory delta (same source)

Substitutions are modeled as a removal plus an addition, not a special type.

Modifier groups carry selection rules: minimum selections, maximum selections,
whether required, whether choices are exclusive, and default selections.

### 5.4 Menu snapshot

The projection order-capture clients actually consume. Denormalized,
immutable, versioned.

Contains every menu item and every modifier with its price and its
**precomputed macro delta**, flattened so a client can do simple addition
locally with no server round-trip.

**Rules**

- Snapshots are immutable. Editing a menu produces a new version.
- Every order records the snapshot version it was captured against.
- Clients cache the snapshot and continue operating on it while offline.

### 5.5 Order

- An order references a menu snapshot version.
- Each order line records: the menu item, the selected modifiers, the quantity,
  the resolved price, and the **resolved macros captured at the time of sale**.
- Resolved macros are stored on the order line, not recomputed on read.

### 5.6 Receipt token and claims

- On order completion, `ordering` issues a receipt token: opaque, unguessable,
  not sequential, not derived from the order id.
- The token resolves to an itemized macro breakdown — items, quantities, and
  per-item macros. It must **not** expose prices, staff, payment details,
  customer identity, or anything else about the order.
- Multiple independent claims may exist against one token — that is the
  shared-table mechanic and it is a feature, not an edge case.
- Claims are anonymous. Do not require an account to claim.
- Tokens should expire after a reasonable window.

---

## 6. Invariants

These hold system-wide. Violating one is a bug regardless of what a task says.

1. **Served nutrition is immutable.** Once an order is closed, its macro values
   are frozen. Recipe changes never rewrite history. A nutrition product whose
   past data silently changes is not a nutrition product.
2. **Macros are never hand-entered above the ingredient level.** Recipes, items,
   and modifiers derive their macros from the ingredient graph. The only place a
   human types a macro number is on an ingredient record.
3. **Money is stored as integer minor units.** Never floating point. Currency is
   explicit on every monetary value.
4. **Quantities are stored in base units.** Conversion happens at the edges.
5. **Every domain row is scoped to a location.** No query may cross tenant
   boundaries without an explicit, audited administrative path.
6. **Order capture is idempotent.** Clients generate the order id; retries and
   offline replays must never create duplicate checks.
7. **The ledger balances.** Every posting has equal debits and credits. Journal
   entries are append-only; corrections are reversing entries, never edits.
8. **Inventory movements are append-only.** On-hand is derived from the movement
   log, not stored as a mutable counter that gets patched.
9. **Nutrition is always presented as an estimate.** No surface may imply exact
   values.
10. **Allergens are never asserted as safe.** If allergen data is added later,
    it may flag presence but must never certify absence. This is the one place
    where liability is genuinely serious.

---

## 7. Nutrition computation

The `nutrition` module owns all of this. No other module computes macros.

**Roll-up.** For a recipe, sum each line item's contribution:
convert the line quantity to the ingredient's base unit, apply the relevant
yield factor if the line uses a prepped form, multiply by the ingredient's
per-base-unit macros. Nested recipes recurse.

**Portioning.** Recipe totals divide by yield/serving count to give per-serving
values. Store both; different consumers need different ones.

**Modifiers.** Compute the delta from the modifier's ingredient delta list using
the same roll-up path. Never store a hand-entered macro delta.

**Order resolution.** Order line macros = item macros + sum of selected modifier
deltas, multiplied by quantity. Resolve once, at sale time, and persist.

**Confidence.** Every computed value carries a confidence signal derived from
the provenance of its inputs — unverified ingredients, missing yield factors,
and modifiers without quantities all degrade it. Surface low confidence in the
back office so operators can fix their data.

**Known sources of the ~10% error**, which should be documented in the UI rather
than engineered away: unmeasured portions (a "drizzle", a hand scoop), prep
variation between staff, ingredient sourcing variation, and modifiers configured
for price without a real quantity.

---

## 8. Client applications

### POS terminal

The highest-traffic, tightest-constraint surface. Responsibilities: order
capture, tender, kitchen fire, receipt issuance, staff session and shift
management.

- **Must function offline.** Orders queue locally and sync on reconnect using
  client-generated ids for idempotency.
- **Must not call the nutrition engine per interaction.** It adds precomputed
  deltas from the cached menu snapshot.
- Circle-specific affordance: a live macro readout as the check is built. Decide
  explicitly whether this is staff-facing only or also on a customer display.
- Thin. No business logic. This is the client most likely to accumulate rot.

### Kitchen display

Consumes fired order lines, groups by station, tracks prep state and timing.
Read-mostly, real-time. No nutrition surface.

### Back office console

Where the ingredient master, recipes, menus, inventory, purchasing, accounting,
labor, and reporting are managed. Onboarding lives here — and **onboarding
friction is the single biggest adoption risk in this domain**, so ingredient
entry and menu mapping deserve disproportionate design effort.

### Diner scan

Public, unauthenticated, mobile-first. Resolves a receipt token, lets a person
select the items they ate, shows their macro totals, and exports them.
Treat all input as hostile. Expose nothing beyond itemized macros.

---

## 9. Conventions

**API.** Versioned. Resource-oriented. Client-generated ids for anything
created from an offline-capable surface. Explicit currency and unit on every
value. Errors carry a stable machine-readable code.

**Events.** Domain events are published through a transactional outbox — never
by writing directly to a message bus inside a business transaction. Events are
facts about the past, named accordingly (`OrderClosed`, not `CloseOrder`).
Consumers must be idempotent.

**Migrations.** Forward-only, reversible where practical, one migration per
change, never edited after being applied. Module schemas are separate; a
migration never touches another module's schema.

**Testing.** The nutrition roll-up, modifier deltas, the GL, inventory
depletion, and unit conversion are the parts where correctness actually matters
— they need thorough unit tests including adversarial cases (zero quantities,
negative deltas, deep nesting, cycles, unit mismatches). Integration tests
should cover the full order lifecycle end to end. Client UI tests are low value
here; skip them.

**Time.** Store UTC. Restaurants have business days that do not align with
calendar days — the business date of an order is a first-class concept, not
`date(created_at)`.

---

## 10. Suggested repository layout

```
/cmd/circle              entrypoint, wiring, config
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
/internal/platform       db, events/outbox, http, config, logging
/migrations              per-module subdirectories
/clients/pos
/clients/kds
/clients/backoffice
/clients/scan
/docs                    decisions, domain notes
```

Each module directory owns its handlers, service logic, repository, and domain
types. Nothing outside a module imports its internals.

---

## 11. Build order

Vertical slices, not complete modules. A thin path through every layer teaches
where the model is wrong before a large amount of code depends on it. Building
the POS to completion before touching inventory means discovering the inventory
implications a year too late.

1. **Core spine.** Ingredient master with units, conversions, and macros.
   Recipe composition with roll-up. Modifiers with real ingredient deltas.
   This is the foundation; get it right before anything else exists.
2. **First vertical slice.** Ingredient → recipe → menu item → order → receipt
   token → inventory depletion → basic cost and macro reporting. Thin in every
   layer, alive end to end.
3. **Menu snapshot and offline order capture.** Versioning, sync, idempotency.
4. **Diner scan.** Token resolution, claims, macro export.
5. **Purchasing and cost.** Vendors, POs, invoices, cost flowing back to
   ingredients.
6. **Accounting.** Double-entry GL, postings from sales, depletion, purchases.
7. **Labor.** Scheduling, clock, labor cost.
8. **Reporting depth.** Menu engineering, theoretical vs actual variance,
   nutrition analytics.

---

## 12. Open questions

Not yet decided. Do not assume an answer; surface the question if a task
depends on one.

- Does the live macro readout appear on a customer-facing display, or staff-only?
- How are ingredients onboarded without tedious manual entry — prebuilt library,
  supplier feeds, assisted mapping, or a combination?
- Does the reporting store stay in Postgres or split out, and at what threshold?
- How are recipes versioned relative to menu snapshots — independently, or does
  any recipe change force a new snapshot?
- What is the claim expiry window, and can a claim be revised after submission?

---

## 13. Guidance for agents

- Prefer correcting the domain model over adding a workaround. This project's
  value is model coherence; a clever patch that violates §4.3 or §6 is a
  regression even if it passes tests.
- When a task is ambiguous, state the assumption in the code review notes rather
  than silently choosing.
- Do not add dependencies casually. This is a solo prototype; every dependency
  is a maintenance obligation.
- Do not build the non-goals in §2, even if a task seems to ask for it. Stub and
  flag instead.
- Keep clients thin. If logic is being added to a client, it belongs on the
  server.
- When touching nutrition, assume the numbers will be shown to someone who is
  making dietary decisions with them. Accuracy of the computation matters even
  though the underlying data is approximate.
- Follow the phase workflow strictly:
  1. implement the current phase
  2. run Snyk scans
  3. debug and fix according to the Snyk report
  4. run Snyk again
  5. repeat the debug/scan loop until the phase is at zero reported bugs
  6. only then commit the phase changes
- Do not commit a phase before the Snyk loop is finished unless explicitly told
  to do so.
- The final phase commit must include the final Snyk report summary in the
  commit message body or description, so the commit explains what was scanned
  and that the final pass was clean.
- Treat Snyk as part of the phase exit criteria, not as an optional later pass.
