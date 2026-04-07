-- ─── 008: Hierarchical categories + product attributes ───────────────────────

-- 1. Иерархия категорий
ALTER TABLE categories ADD COLUMN IF NOT EXISTS parent_id BIGINT REFERENCES categories(id) ON DELETE SET NULL;
ALTER TABLE categories ADD COLUMN IF NOT EXISTS sort_order INTEGER NOT NULL DEFAULT 0;

-- 2. Определения атрибутов (схема) — принадлежат категории
CREATE TABLE IF NOT EXISTS attr_defs (
    id          BIGSERIAL PRIMARY KEY,
    category_id BIGINT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    slug        VARCHAR(255) NOT NULL,
    value_type  VARCHAR(20)  NOT NULL DEFAULT 'string',  -- 'string' | 'number'
    sort_order  INTEGER NOT NULL DEFAULT 0,
    UNIQUE(category_id, slug)
);

-- 3. Значения атрибутов конкретного товара
CREATE TABLE IF NOT EXISTS attr_values (
    id          BIGSERIAL PRIMARY KEY,
    product_id  BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    attr_def_id BIGINT NOT NULL REFERENCES attr_defs(id) ON DELETE CASCADE,
    value_str   TEXT,
    value_num   NUMERIC(18, 4),
    UNIQUE(product_id, attr_def_id)
);

CREATE INDEX IF NOT EXISTS idx_attr_values_product  ON attr_values(product_id);
CREATE INDEX IF NOT EXISTS idx_attr_values_def      ON attr_values(attr_def_id);
CREATE INDEX IF NOT EXISTS idx_attr_defs_category   ON attr_defs(category_id);
CREATE INDEX IF NOT EXISTS idx_categories_parent    ON categories(parent_id);
