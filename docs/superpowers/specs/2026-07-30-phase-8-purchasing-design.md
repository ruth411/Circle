# Phase 8 Purchasing Design

## Goal

Build the first operational purchasing slice so a location can maintain
vendors, map vendor packs to ingredients, create purchase orders, receive
stock against PO lines, increase inventory from receipts, and update
ingredient cost from actual received cost.

## Scope Decision

Phase 8 stops at operational purchasing.

Included:

- vendors
- vendor items
- purchase orders
- purchase order lines
- receipts
- receipt lines
- inventory receipt posting
- ingredient cost updates from receipts

Excluded and deferred to Phase 9:

- vendor invoices
- invoice matching
- AP aging
- vendor payment workflow

## Why This Scope

This keeps the phase on the operational truth: what was ordered, what arrived,
what entered stock, and what it actually cost. That is enough to connect
purchasing back into inventory and ingredient cost without dragging accounting
workflow into the same implementation.

## Domain Boundaries

`purchasing` owns vendor master data, vendor item mappings, purchase orders,
and receipts.

`inventory` remains the source of truth for stock on hand and stock movement.
Purchasing does not mutate stock tables directly in ad hoc ways. It asks
inventory to record a receipt movement from immutable receipt data.

`ingredient` remains the source of truth for ingredient master data and cost
fields. Purchasing can update cost fields through a defined repository/service
path, but it does not take ownership of ingredient records.

`accounting` is not part of this phase. Finance-oriented workflows stay out.

## Data Model

Add tables under the `purchasing` schema:

### `vendors`

Purpose: a supplier a location buys from.

Required fields:

- `location_id`
- `id`
- `name`
- `status` (`active` or `inactive`)
- `external_ref` nullable
- `created_at`
- `updated_at`

Non-goals:

- no contact-person subsystem
- no addresses subsystem
- no payment terms engine

### `vendor_items`

Purpose: map what a vendor sells to one Circle ingredient.

Required fields:

- `location_id`
- `id`
- `vendor_id`
- `ingredient_id`
- `vendor_sku` nullable
- `name`
- `purchase_unit`
- `pack_quantity`
- `ingredient_base_quantity`
- `last_unit_cost`
- `currency`
- `status`
- `created_at`
- `updated_at`

Meaning:

- `purchase_unit` describes the unit the vendor sells in, such as `case` or
  `bag`
- `pack_quantity` describes how many purchase units are represented
- `ingredient_base_quantity` is the inventory quantity produced in the linked
  ingredient base unit

Example:

- vendor sells one case of chicken
- `purchase_unit = case`
- `pack_quantity = 1`
- `ingredient_base_quantity = 40000`
- ingredient base unit = grams

### `purchase_orders`

Purpose: the PO header for one vendor at one location.

Required fields:

- `location_id`
- `id`
- `po_number`
- `vendor_id`
- `status`
- `ordered_at` nullable
- `notes` nullable
- `created_at`
- `updated_at`

Statuses:

- `draft`
- `submitted`
- `partially_received`
- `received`
- `cancelled`

Rules:

- `po_number` is unique per location, not globally
- only `draft` POs are editable as drafts
- `cancelled` POs cannot be received

### `purchase_order_lines`

Purpose: the ordered units and expected cost.

Required fields:

- `location_id`
- `id`
- `purchase_order_id`
- `vendor_item_id`
- `ordered_quantity`
- `ordered_unit_cost`
- `currency`
- `received_quantity`
- `created_at`
- `updated_at`

Rules:

- line totals are derived, not separately trusted
- `received_quantity` is cumulative and cannot exceed `ordered_quantity` in
  Phase 8

### `receipts`

Purpose: a single receiving event for a PO.

Required fields:

- `location_id`
- `id`
- `purchase_order_id`
- `received_at`
- `received_by` nullable
- `notes` nullable
- `created_at`

Rules:

- receipt creation is idempotent by `(location_id, id)`
- one receipt can partially receive many PO lines

### `receipt_lines`

Purpose: immutable received quantities and actual costs.

Required fields:

- `location_id`
- `id`
- `receipt_id`
- `purchase_order_line_id`
- `received_quantity`
- `received_unit_cost`
- `currency`
- `inventory_quantity`
- `inventory_unit`

Rules:

- `inventory_quantity` is resolved and frozen in the ingredient base unit at
  receipt time
- `inventory_unit` is frozen with that quantity so later ingredient-unit edits
  do not corrupt history

## Service Behavior

### Vendor management

Provide create, update, list, and lookup flows.

Validation:

- location ID required
- vendor name required after trim
- inactive vendors remain readable but cannot be used on new POs

### Vendor item mapping

Provide create, update, list, and lookup flows.

Validation:

- location ID required
- vendor and ingredient must belong to the same location
- conversion quantities must be positive
- currency must match existing project money rules

### Purchase order flow

Provide:

- create PO header
- add/update/remove draft lines
- submit PO
- list/get PO

Validation:

- vendor must be active
- vendor item must belong to the PO location
- only `draft` POs accept draft edits
- submit requires at least one line

### Receiving flow

Provide:

- create receipt for a submitted or partially received PO
- receive quantity per PO line
- accept actual unit cost at receive time

Validation:

- cannot receive a `draft` PO
- cannot receive a `cancelled` PO
- cannot receive more than remaining ordered quantity in Phase 8
- every receipt line must target a PO line from the same PO
- receipt creation must be atomic with inventory movement and PO status update

Status rules:

- if no lines received yet after submit: `submitted`
- if some but not all ordered quantity received: `partially_received`
- if all lines fully received: `received`

## Inventory Integration

Receipt posting is the reason this phase exists.

When a receipt is created:

1. Resolve each vendor item into the linked ingredient
2. Convert received purchase quantity into ingredient base quantity
3. Insert receipt and receipt lines
4. Record inventory movement of type `receipt`
5. Update PO line cumulative received quantity
6. Recompute PO status
7. Update ingredient cost fields

All of that should happen in one transaction so stock, PO status, and receipt
history do not drift apart.

Inventory movement must be written from the immutable receipt snapshot, not
from mutable current vendor or ingredient metadata looked up later.

## Cost Update Policy

Phase 8 keeps cost updates deliberately small.

Required:

- update ingredient `LastReceivedCost`
- update ingredient `LastReceivedAt`

Optional only if the current ingredient model already has a clean place for
it:

- weighted-average cost

Recommendation:

Ship `LastReceivedCost` first. Do not force weighted-average cost into this
phase unless the current ingredient code already supports it naturally.

## HTTP/API Slice

Add staff-only endpoints for:

- vendors list/create/update
- vendor items list/create/update
- purchase orders list/create/get
- purchase order draft-line edits
- purchase order submit
- receipts create/get/list

Out of scope:

- public endpoints
- PDF/email vendor outputs
- reporting endpoints

## Error Handling

Rules that matter:

- duplicate create IDs should behave consistently with existing location-scoped
  idempotency patterns
- cross-location references should fail closed
- invalid state transitions should return domain errors, not raw SQL errors
- partial receive attempts that exceed remaining quantity must fail before any
  stock movement is posted

## Testing

Minimum required tests:

- vendor create/update/list happy path
- vendor item rejects cross-location vendor or ingredient references
- PO draft create and line edit flow
- PO submit requires at least one line
- submitted PO rejects draft edits
- partial receipt updates line balances and PO status correctly
- full receipt closes the PO correctly
- duplicate receipt ID is idempotent
- receipt posts inventory exactly once
- receipt line stores frozen inventory quantity and unit
- ingredient `LastReceivedCost` updates from actual received cost
- all APIs and repository paths enforce location scoping

## Exit Criteria

Phase 8 is complete when:

- a location can maintain vendors
- a location can maintain vendor item mappings into ingredients
- a location can create and submit a purchase order
- a location can receive stock against PO lines
- stock increases in inventory base units correctly from receipt posting
- ingredient cost fields update from actual received cost
- all flows are location-scoped, atomic, and idempotent where required
