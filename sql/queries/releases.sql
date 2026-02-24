-- name: UpsertRelease :exec
INSERT INTO releases (version, from_ver, platform, release_date, submitted_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, now(), now())
ON CONFLICT (version) DO UPDATE SET
    from_ver = EXCLUDED.from_ver,
    platform = EXCLUDED.platform,
    release_date = EXCLUDED.release_date,
    submitted_by = EXCLUDED.submitted_by,
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
