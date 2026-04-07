-- SEO fields for categories
ALTER TABLE categories ADD COLUMN IF NOT EXISTS seo_title       TEXT NOT NULL DEFAULT '';
ALTER TABLE categories ADD COLUMN IF NOT EXISTS seo_description TEXT NOT NULL DEFAULT '';

-- SEO fields for products
ALTER TABLE products ADD COLUMN IF NOT EXISTS seo_title       TEXT NOT NULL DEFAULT '';
ALTER TABLE products ADD COLUMN IF NOT EXISTS seo_description TEXT NOT NULL DEFAULT '';

-- Global site settings (key-value)
CREATE TABLE IF NOT EXISTS site_settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);

INSERT INTO site_settings (key, value) VALUES
  ('site_name',        'Magaz'),
  ('site_description', 'Современный интернет-магазин'),
  ('site_keywords',    ''),
  ('og_image',         ''),
  ('robots_txt',       E'User-agent: *\nAllow: /\nDisallow: /admin/\nSitemap: /sitemap.xml')
ON CONFLICT (key) DO NOTHING;
