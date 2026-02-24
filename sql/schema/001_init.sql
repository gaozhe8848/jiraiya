CREATE TABLE IF NOT EXISTS jiras (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL DEFAULT '',
    impact TEXT NOT NULL DEFAULT '',
    domain TEXT NOT NULL DEFAULT '',
    relnotes TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS releases (
    version TEXT PRIMARY KEY,
    from_ver TEXT NOT NULL DEFAULT '',
    platform TEXT NOT NULL DEFAULT '',
    release_date TEXT NOT NULL DEFAULT '',
    submitted_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_releases_platform ON releases(platform);

CREATE TABLE IF NOT EXISTS release_jiras (
    release_version TEXT NOT NULL REFERENCES releases(version) ON DELETE CASCADE,
    jira_id TEXT NOT NULL REFERENCES jiras(id) ON DELETE CASCADE,
    PRIMARY KEY (release_version, jira_id)
);
CREATE INDEX IF NOT EXISTS idx_release_jiras_jira_id ON release_jiras(jira_id);
