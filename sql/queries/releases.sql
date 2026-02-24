-- name: UpsertRelease :exec
INSERT INTO releases (version, from_ver, platform, release_date, submitted_by, path, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now(), now())
ON CONFLICT (version) DO UPDATE SET
    from_ver = EXCLUDED.from_ver,
    platform = EXCLUDED.platform,
    release_date = EXCLUDED.release_date,
    submitted_by = EXCLUDED.submitted_by,
    path = EXCLUDED.path,
    updated_at = now();

-- name: GetRelease :one
SELECT version, from_ver, platform, release_date, submitted_by, created_at, updated_at
FROM releases
WHERE version = $1;

-- name: DeleteRelease :exec
DELETE FROM releases WHERE version = $1;

-- name: GetVersionsByPlatform :many
SELECT version, from_ver, release_date, submitted_by, created_at, updated_at
FROM releases
WHERE platform = $1
ORDER BY release_date DESC;

-- name: GetAllReleasesByPlatform :many
SELECT version, from_ver, platform, release_date, submitted_by, created_at, updated_at
FROM releases
WHERE platform = $1;

-- name: GetAllPlatforms :many
SELECT DISTINCT platform FROM releases WHERE platform != '' ORDER BY platform;

-- name: GetReleasePath :one
SELECT path::text FROM releases WHERE version = $1;

-- name: CountPathAncestors :one
SELECT count(*) FROM releases WHERE path @> $1::ltree;

-- name: CalcChgs :many
WITH paths AS (
  SELECT
    (SELECT r1.path FROM releases r1 WHERE r1.version = @to_version) AS to_path,
    (SELECT r2.path FROM releases r2 WHERE r2.version = @from_version) AS from_path
),
lca AS (
  SELECT lca(p.to_path, p.from_path) AS lca_path FROM paths p
)
SELECT rj.jira_id FROM release_jiras rj
JOIN releases r ON r.version = rj.release_version
WHERE r.path @> (SELECT p.to_path FROM paths p)
  AND nlevel(r.path) > nlevel((SELECT l.lca_path FROM lca l))
EXCEPT
SELECT rj.jira_id FROM release_jiras rj
JOIN releases r ON r.version = rj.release_version
WHERE r.path @> (SELECT p.from_path FROM paths p)
  AND nlevel(r.path) > nlevel((SELECT l.lca_path FROM lca l))
ORDER BY jira_id;
