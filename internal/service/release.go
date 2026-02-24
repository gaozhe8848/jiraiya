package service

import (
	"context"
	"fmt"
	"regexp"

	"jiraiya/internal/db"
)

// ValidationError holds details about invalid jiras in a submission.
type ValidationError struct {
	Details []ValidationDetail `json:"details"`
}

// ValidationDetail describes a single validation failure.
type ValidationDetail struct {
	Index  int    `json:"index"`
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

func (e *ValidationError) Error() string {
	return "validation failed"
}

var sanitizeRe = regexp.MustCompile(`[^A-Za-z0-9]`)

func sanitizeLabel(version string) string {
	return sanitizeRe.ReplaceAllString(version, "_")
}

func (s *svc) SubmitRelease(ctx context.Context, sub ReleaseSubmission) error {
	// Validate release
	r := sub.Release
	if r.Version == "" {
		return &ValidationError{Details: []ValidationDetail{{Reason: "release version is required"}}}
	}
	if r.Platform == "" {
		return &ValidationError{Details: []ValidationDetail{{Reason: "release platform is required"}}}
	}

	// Validate jiras
	var details []ValidationDetail
	for i, j := range sub.Changes {
		if j.ID == "" {
			details = append(details, ValidationDetail{Index: i, ID: j.ID, Reason: "jira id is required"})
		}
	}
	if len(details) > 0 {
		return &ValidationError{Details: details}
	}

	// Begin transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.q.WithTx(tx)

	// Compute ltree path
	var path string
	label := sanitizeLabel(r.Version)
	if r.FromVer == "" {
		path = label
	} else {
		parentPath, err := qtx.GetReleasePath(ctx, r.FromVer)
		if err != nil {
			return fmt.Errorf("get parent path for %s: %w", r.FromVer, err)
		}
		path = parentPath + "." + label
	}

	// Upsert each jira
	for _, j := range sub.Changes {
		if err := qtx.UpsertJira(ctx, db.UpsertJiraParams{
			ID:       j.ID,
			Title:    j.Title,
			Impact:   j.Impact,
			Domain:   j.Domain,
			Relnotes: j.Relnotes,
		}); err != nil {
			return fmt.Errorf("upsert jira %s: %w", j.ID, err)
		}
	}

	// Upsert release
	if err := qtx.UpsertRelease(ctx, db.UpsertReleaseParams{
		Version:     r.Version,
		FromVer:     r.FromVer,
		Platform:    r.Platform,
		ReleaseDate: r.ReleaseDate,
		SubmittedBy: r.SubmittedBy,
		Path:        path,
	}); err != nil {
		return fmt.Errorf("upsert release: %w", err)
	}

	// Unlink old jiras, re-link new ones
	if err := qtx.UnlinkJirasFromRelease(ctx, r.Version); err != nil {
		return fmt.Errorf("unlink jiras: %w", err)
	}
	for _, j := range sub.Changes {
		if err := qtx.LinkJiraToRelease(ctx, db.LinkJiraToReleaseParams{
			ReleaseVersion: r.Version,
			JiraID:         j.ID,
		}); err != nil {
			return fmt.Errorf("link jira %s: %w", j.ID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	s.log.Info("release submitted", "version", r.Version, "submitted_by", r.SubmittedBy, "jira_count", len(sub.Changes))
	return nil
}

func (s *svc) DeleteRelease(ctx context.Context, version string) error {
	rel, err := s.q.GetRelease(ctx, version)
	if err != nil {
		return fmt.Errorf("get release %s: %w", version, err)
	}

	if err := s.q.DeleteRelease(ctx, version); err != nil {
		return fmt.Errorf("delete release %s: %w", version, err)
	}

	s.log.Info("release deleted", "version", version, "platform", rel.Platform)
	return nil
}
