package service

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"jiraiya/internal/db"
)

// JiraInput is a single jira from the PUT request body.
type JiraInput struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Domain   string `json:"domain"`
	Impact   string `json:"impact"`
	Relnotes string `json:"relnotes"`
}

// ReleaseInfo is the release metadata from the PUT request body.
type ReleaseInfo struct {
	Version     string `json:"version"`
	FromVer     string `json:"from_ver"`
	Platform    string `json:"platform"`
	ReleaseDate string `json:"release_date"`
	SubmittedBy string `json:"submitted_by"`
}

// ReleaseSubmission is the full PUT request body.
type ReleaseSubmission struct {
	Changes []JiraInput `json:"changes"`
	Release ReleaseInfo `json:"release"`
}

// Filters holds the distinct domain and impact values for a platform.
type Filters struct {
	Domains []string `json:"domains"`
	Impacts []string `json:"impacts"`
}

// VersionInfo is a release version returned by GetVersions.
type VersionInfo struct {
	Version     string `json:"version"`
	FromVer     string `json:"from_ver"`
	ReleaseDate string `json:"release_date"`
	SubmittedBy string `json:"submitted_by"`
}

// ReleaseOutput is a release returned to the client.
type ReleaseOutput struct {
	Version     string `json:"version"`
	FromVer     string `json:"from_ver"`
	Platform    string `json:"platform"`
	ReleaseDate string `json:"release_date"`
	SubmittedBy string `json:"submitted_by"`
}

// JiraOutput is a jira returned to the client.
type JiraOutput struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Impact   string `json:"impact"`
	Domain   string `json:"domain"`
	Relnotes string `json:"relnotes"`
}

// NodeInfo represents a single node in the tree dump.
type NodeInfo struct {
	Version  string   `json:"version"`
	FromVer  string   `json:"from_ver"`
	Changes  []string `json:"changes"`
	Children []string `json:"children"`
}

// TreeInfo is the admin tree introspection response.
type TreeInfo struct {
	Platform  string     `json:"platform"`
	NodeCount int        `json:"node_count"`
	Root      string     `json:"root"`
	Nodes     []NodeInfo `json:"nodes"`
}

// Service defines the business logic interface.
type Service interface {
	SubmitRelease(ctx context.Context, sub ReleaseSubmission) error
	DeleteRelease(ctx context.Context, version string) error
	GetReleases(ctx context.Context, version, platform string) ([]ReleaseOutput, error)
	GetFilters(ctx context.Context, platform string) (*Filters, error)
	GetVersions(ctx context.Context, platform string) ([]VersionInfo, error)
	GetJirasBetweenVersions(ctx context.Context, fromVer, toVer string) ([]JiraOutput, error)
	GetTreeInfo(ctx context.Context, platform string) (*TreeInfo, error)
}

type svc struct {
	pool *pgxpool.Pool
	q    *db.Queries
	log  *slog.Logger
}

// New creates a new Service backed by the given pool.
func New(pool *pgxpool.Pool, log *slog.Logger) Service {
	return &svc{
		pool: pool,
		q:    db.New(pool),
		log:  log,
	}
}
