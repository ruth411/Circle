ALTER TABLE recipe.menu_snapshot_items
    ADD COLUMN IF NOT EXISTS ingredient_units_json JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE recipe.menu_snapshot_modifiers
    ADD COLUMN IF NOT EXISTS ingredient_units_json JSONB NOT NULL DEFAULT '{}'::jsonb;

DROP TRIGGER IF EXISTS recipe_menu_snapshot_items_immutable ON recipe.menu_snapshot_items;
DROP TRIGGER IF EXISTS recipe_menu_snapshot_modifiers_immutable ON recipe.menu_snapshot_modifiers;

UPDATE recipe.menu_snapshot_items AS items
SET ingredient_units_json = COALESCE((
    SELECT jsonb_object_agg(usage.key, to_jsonb(ingredients.base_unit))
    FROM recipe.menu_snapshots AS snapshots
    JOIN jsonb_each(items.ingredient_usage_json) AS usage ON TRUE
    JOIN ingredient.ingredients AS ingredients
      ON ingredients.id = usage.key
     AND ingredients.location_id = snapshots.location_id
    WHERE snapshots.id = items.snapshot_id
), '{}'::jsonb)
WHERE items.ingredient_units_json = '{}'::jsonb;

UPDATE recipe.menu_snapshot_modifiers AS modifiers
SET ingredient_units_json = COALESCE((
    SELECT jsonb_object_agg(usage.key, to_jsonb(ingredients.base_unit))
    FROM recipe.menu_snapshot_modifier_groups AS groups
    JOIN recipe.menu_snapshots AS snapshots
      ON snapshots.id = groups.snapshot_id
    JOIN jsonb_each(modifiers.ingredient_usage_json) AS usage ON TRUE
    JOIN ingredient.ingredients AS ingredients
      ON ingredients.id = usage.key
     AND ingredients.location_id = snapshots.location_id
    WHERE groups.snapshot_id = modifiers.snapshot_id
      AND groups.group_id = modifiers.group_id
), '{}'::jsonb)
WHERE modifiers.ingredient_units_json = '{}'::jsonb;

CREATE TRIGGER recipe_menu_snapshot_items_immutable
    BEFORE UPDATE OR DELETE ON recipe.menu_snapshot_items
    FOR EACH ROW
    EXECUTE FUNCTION recipe.prevent_snapshot_mutation();

CREATE TRIGGER recipe_menu_snapshot_modifiers_immutable
    BEFORE UPDATE OR DELETE ON recipe.menu_snapshot_modifiers
    FOR EACH ROW
    EXECUTE FUNCTION recipe.prevent_snapshot_mutation();

ALTER TABLE ordering.order_lines
    ADD COLUMN IF NOT EXISTS ingredient_units_json JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE ordering.order_line_modifiers
    ADD COLUMN IF NOT EXISTS ingredient_units_json JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE ordering.order_lines AS lines
SET ingredient_units_json = COALESCE((
    SELECT snapshot_items.ingredient_units_json
    FROM ordering.orders AS orders
    LEFT JOIN recipe.menu_snapshot_items AS snapshot_items
      ON snapshot_items.snapshot_id = orders.snapshot_id
     AND snapshot_items.menu_item_id = lines.menu_item_id
    WHERE orders.location_id = lines.location_id
      AND orders.id = lines.order_id
), '{}'::jsonb)
WHERE lines.ingredient_units_json = '{}'::jsonb;

UPDATE ordering.order_line_modifiers AS modifiers
SET ingredient_units_json = COALESCE((
    SELECT snapshot_modifiers.ingredient_units_json
    FROM ordering.orders AS orders
    LEFT JOIN recipe.menu_snapshot_modifiers AS snapshot_modifiers
      ON snapshot_modifiers.snapshot_id = orders.snapshot_id
     AND snapshot_modifiers.modifier_id = modifiers.modifier_id
    WHERE orders.location_id = modifiers.location_id
      AND orders.id = modifiers.order_id
), '{}'::jsonb)
WHERE modifiers.ingredient_units_json = '{}'::jsonb;
