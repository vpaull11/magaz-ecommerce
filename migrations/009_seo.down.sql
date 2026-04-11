DROP TABLE IF EXISTS site_settings;
ALTER TABLE products DROP COLUMN IF EXISTS seo_description;
ALTER TABLE products DROP COLUMN IF EXISTS seo_title;
ALTER TABLE categories DROP COLUMN IF EXISTS seo_description;
ALTER TABLE categories DROP COLUMN IF EXISTS seo_title;
