CREATE EXTENSION IF NOT EXISTS ltree;

ALTER TABLE releases ADD COLUMN IF NOT EXISTS path ltree;
CREATE INDEX IF NOT EXISTS idx_releases_path ON releases USING gist(path);

-- Populate paths for existing rows.
-- Labels: sanitize version names (replace non-alphanumeric with '_').
WITH RECURSIVE tree AS (
  SELECT version, from_ver,
    regexp_replace(version, '[^A-Za-z0-9]', '_', 'g')::ltree AS path
  FROM releases WHERE from_ver = ''
  UNION ALL
  SELECT r.version, r.from_ver,
    (t.path || regexp_replace(r.version, '[^A-Za-z0-9]', '_', 'g'))::ltree
  FROM releases r JOIN tree t ON r.from_ver = t.version
)
UPDATE releases SET path = tree.path FROM tree
WHERE releases.version = tree.version;

ALTER TABLE releases ALTER COLUMN path SET NOT NULL;
