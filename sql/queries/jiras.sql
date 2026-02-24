-- name: UpsertJira :exec
INSERT INTO jiras (id, title, impact, domain, relnotes)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO UPDATE SET
    title = EXCLUDED.title,
    impact = EXCLUDED.impact,
    domain = EXCLUDED.domain,
    relnotes = EXCLUDED.relnotes;

-- name: GetJirasByIDs :many
SELECT id, title, impact, domain, relnotes
FROM jiras
WHERE id = ANY(@ids::text[]);

-- name: GetDistinctDomains :many
SELECT DISTINCT j.domain
FROM jiras j
JOIN release_jiras rj ON rj.jira_id = j.id
JOIN releases r ON r.version = rj.release_version
WHERE r.platform = $1 AND j.domain != ''
ORDER BY j.domain;

-- name: GetDistinctImpacts :many
SELECT DISTINCT j.impact
FROM jiras j
JOIN release_jiras rj ON rj.jira_id = j.id
JOIN releases r ON r.version = rj.release_version
WHERE r.platform = $1 AND j.impact != ''
ORDER BY j.impact;
