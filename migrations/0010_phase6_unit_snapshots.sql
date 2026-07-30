ALTER TABLE recipe.menu_snapshot_items
    ADD COLUMN IF NOT EXISTS ingredient_units_json JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE recipe.menu_snapshot_modifiers
    ADD COLUMN IF NOT EXISTS ingredient_units_json JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE recipe.menu_snapshot_items AS items
SET ingredient_units_json = COALESCE(units.units_json, '{}'::jsonb)
FROM recipe.menu_snapshots AS snapshots
LEFT JOIN LATERAL (
    SELECT jsonb_object_agg(usage.key, to_jsonb(ingredients.base_unit)) AS units_json
    FROM jsonb_each(items.ingredient_usage_json) AS usage
    JOIN ingredient.ingredients AS ingredients
      ON ingredients.id = usage.key
     AND ingredients.location_id = snapshots.location_id
) AS units ON TRUE
WHERE snapshots.id = items.snapshot_id
  AND items.ingredient_units_json = '{}'::jsonb;

UPDATE recipe.menu_snapshot_modifiers AS modifiers
SET ingredient_units_json = COALESCE(units.units_json, '{}'::jsonb)
FROM recipe.menu_snapshot_modifier_groups AS groups
JOIN recipe.menu_snapshots AS snapshots
  ON snapshots.id = groups.snapshot_id
LEFT JOIN LATERAL (
    SELECT jsonb_object_agg(usage.key, to_jsonb(ingredients.base_unit)) AS units_json
    FROM jsonb_each(modifiers.ingredient_usage_json) AS usage
    JOIN ingredient.ingredients AS ingredients
      ON ingredients.id = usage.key
     AND ingredients.location_id = snapshots.location_id
) AS units ON TRUE
WHERE groups.snapshot_id = modifiers.snapshot_id
  AND groups.group_id = modifiers.group_id
  AND modifiers.ingredient_units_json = '{}'::jsonb;

ALTER TABLE ordering.order_lines
    ADD COLUMN IF NOT EXISTS ingredient_units_json JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE ordering.order_line_modifiers
    ADD COLUMN IF NOT EXISTS ingredient_units_json JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE ordering.order_lines AS lines
SET ingredient_units_json = COALESCE(snapshot_items.ingredient_units_json, '{}'::jsonb)
FROM ordering.orders AS orders
LEFT JOIN recipe.menu_snapshot_items AS snapshot_items
  ON snapshot_items.snapshot_id = orders.snapshot_id
 AND snapshot_items.menu_item_id = lines.menu_item_id
WHERE orders.location_id = lines.location_id
  AND orders.id = lines.order_id
  AND lines.ingredient_units_json = '{}'::jsonb;

UPDATE ordering.order_line_modifiers AS modifiers
SET ingredient_units_json = COALESCE(snapshot_modifiers.ingredient_units_json, '{}'::jsonb)
FROM ordering.orders AS orders
LEFT JOIN recipe.menu_snapshot_modifiers AS snapshot_modifiers
  ON snapshot_modifiers.snapshot_id = orders.snapshot_id
 AND snapshot_modifiers.modifier_id = modifiers.modifier_id
WHERE orders.location_id = modifiers.location_id
  AND orders.id = modifiers.order_id
  AND modifiers.ingredient_units_json = '{}'::jsonb;
