-- name: LinkJiraToRelease :exec
INSERT INTO release_jiras (release_version, jira_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnlinkJirasFromRelease :exec
DELETE FROM release_jiras WHERE release_version = $1;

-- name: GetJiraIDsByRelease :many
SELECT jira_id FROM release_jiras WHERE release_version = $1;
