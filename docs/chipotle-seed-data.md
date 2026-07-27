# Chipotle Seed Data

Last verified: 2026-07-26

## Purpose

Circle's first seeded menu dataset is based on Chipotle's official public menu
and nutrition experience.

This is a prototype seed, not a live integration.

## Modeling decision

The current Chipotle seed uses **single serving values** from Chipotle's
official nutrition experience.

That means:

- each seeded ingredient uses `base_unit = each`
- one `each` means one official Chipotle serving for that menu ingredient
- the stored macros are the official macros for that one serving

This is deliberate for the prototype.
We are not inventing gram conversions that Chipotle does not publicly expose in
the same clean form.

## Official sources used

- Chipotle Nutrition Calculator:
  https://www.chipotle.com/nutrition-calculator
- Chipotle Order/Menu experience:
  https://www.chipotle.com/order/build/burrito-bowl
- Chipotle public ingredients experience:
  https://www.chipotle.com/ingredients

## Seed scope

The first seed set creates:

- `Chipotle Corporate`
- `Chipotle`
- `Chipotle Charlotte`
- `Chipotle Raleigh`

And it seeds a first ingredient catalog for both locations using the same
official single-serving menu values.

## Important limitation

These rows are **serving-based menu ingredients**, not raw gram-based ingredient
science records.

They are good enough to start the Chipotle prototype cleanly.
If a future official weight-based source becomes available, these rows can be
replaced or expanded with gram/ml-based ingredient records.
