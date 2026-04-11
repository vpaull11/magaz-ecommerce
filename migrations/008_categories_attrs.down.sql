DROP INDEX IF EXISTS idx_categories_parent;
DROP INDEX IF EXISTS idx_attr_defs_category;
DROP INDEX IF EXISTS idx_attr_values_def;
DROP INDEX IF EXISTS idx_attr_values_product;
DROP TABLE IF EXISTS attr_values CASCADE;
DROP TABLE IF EXISTS attr_defs CASCADE;
ALTER TABLE categories DROP COLUMN IF EXISTS sort_order;
ALTER TABLE categories DROP COLUMN IF EXISTS parent_id;
