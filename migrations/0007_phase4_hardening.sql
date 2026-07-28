ALTER TABLE recipe.recipes
    ADD CONSTRAINT recipe_recipes_location_id_unique
    UNIQUE (location_id, id);

ALTER TABLE ingredient.ingredients
    ADD CONSTRAINT ingredient_ingredients_location_id_unique
    UNIQUE (location_id, id);

ALTER TABLE recipe.menu_items
    ADD CONSTRAINT recipe_menu_items_location_id_unique
    UNIQUE (location_id, id);

ALTER TABLE recipe.modifier_groups
    ADD CONSTRAINT recipe_modifier_groups_location_id_unique
    UNIQUE (location_id, id);

ALTER TABLE recipe.modifiers
    ADD CONSTRAINT recipe_modifiers_location_id_unique
    UNIQUE (location_id, id);

ALTER TABLE recipe.menu_items
    DROP CONSTRAINT IF EXISTS recipe_menu_items_recipe_id_fkey;

ALTER TABLE recipe.menu_items
    ADD CONSTRAINT recipe_menu_items_location_recipe_fk
    FOREIGN KEY (location_id, recipe_id)
    REFERENCES recipe.recipes (location_id, id);

ALTER TABLE recipe.modifier_groups
    DROP CONSTRAINT IF EXISTS recipe_modifier_groups_menu_item_id_fkey;

ALTER TABLE recipe.modifier_groups
    ADD CONSTRAINT recipe_modifier_groups_location_menu_item_fk
    FOREIGN KEY (location_id, menu_item_id)
    REFERENCES recipe.menu_items (location_id, id)
    ON DELETE CASCADE;

ALTER TABLE recipe.modifiers
    DROP CONSTRAINT IF EXISTS recipe_modifiers_modifier_group_id_fkey;

ALTER TABLE recipe.modifiers
    ADD CONSTRAINT recipe_modifiers_location_group_fk
    FOREIGN KEY (location_id, modifier_group_id)
    REFERENCES recipe.modifier_groups (location_id, id)
    ON DELETE CASCADE;

ALTER TABLE recipe.modifier_ingredient_deltas
    DROP CONSTRAINT IF EXISTS recipe_modifier_ingredient_deltas_ingredient_id_fkey;

ALTER TABLE recipe.modifier_ingredient_deltas
    ADD CONSTRAINT recipe_modifier_deltas_location_ingredient_fk
    FOREIGN KEY (location_id, ingredient_id)
    REFERENCES ingredient.ingredients (location_id, id);

ALTER TABLE recipe.modifier_ingredient_deltas
    DROP CONSTRAINT IF EXISTS recipe_modifier_ingredient_deltas_modifier_id_fkey;

ALTER TABLE recipe.modifier_ingredient_deltas
    ADD CONSTRAINT recipe_modifier_deltas_location_modifier_fk
    FOREIGN KEY (location_id, modifier_id)
    REFERENCES recipe.modifiers (location_id, id)
    ON DELETE CASCADE;

CREATE OR REPLACE FUNCTION recipe.prevent_snapshot_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'menu snapshots are immutable';
END;
$$;

DROP TRIGGER IF EXISTS recipe_menu_snapshots_immutable ON recipe.menu_snapshots;
CREATE TRIGGER recipe_menu_snapshots_immutable
    BEFORE UPDATE OR DELETE ON recipe.menu_snapshots
    FOR EACH ROW
    EXECUTE FUNCTION recipe.prevent_snapshot_mutation();

DROP TRIGGER IF EXISTS recipe_menu_snapshot_items_immutable ON recipe.menu_snapshot_items;
CREATE TRIGGER recipe_menu_snapshot_items_immutable
    BEFORE UPDATE OR DELETE ON recipe.menu_snapshot_items
    FOR EACH ROW
    EXECUTE FUNCTION recipe.prevent_snapshot_mutation();

DROP TRIGGER IF EXISTS recipe_menu_snapshot_modifier_groups_immutable ON recipe.menu_snapshot_modifier_groups;
CREATE TRIGGER recipe_menu_snapshot_modifier_groups_immutable
    BEFORE UPDATE OR DELETE ON recipe.menu_snapshot_modifier_groups
    FOR EACH ROW
    EXECUTE FUNCTION recipe.prevent_snapshot_mutation();

DROP TRIGGER IF EXISTS recipe_menu_snapshot_modifiers_immutable ON recipe.menu_snapshot_modifiers;
CREATE TRIGGER recipe_menu_snapshot_modifiers_immutable
    BEFORE UPDATE OR DELETE ON recipe.menu_snapshot_modifiers
    FOR EACH ROW
    EXECUTE FUNCTION recipe.prevent_snapshot_mutation();
