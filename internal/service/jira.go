package service

import (
	"context"
	"fmt"
)

func (s *svc) GetReleases(ctx context.Context, version, platform string) ([]ReleaseOutput, error) {
	if version != "" {
		r, err := s.q.GetRelease(ctx, version)
		if err != nil {
			return nil, fmt.Errorf("get release %s: %w", version, err)
		}
		return []ReleaseOutput{{
			Version:     r.Version,
			FromVer:     r.FromVer,
			Platform:    r.Platform,
			ReleaseDate: r.ReleaseDate,
			SubmittedBy: r.SubmittedBy,
		}}, nil
	}

	rows, err := s.q.GetAllReleasesByPlatform(ctx, platform)
	if err != nil {
		return nil, fmt.Errorf("get releases by platform: %w", err)
	}
	out := make([]ReleaseOutput, len(rows))
	for i, r := range rows {
		out[i] = ReleaseOutput{
			Version:     r.Version,
			FromVer:     r.FromVer,
			Platform:    r.Platform,
			ReleaseDate: r.ReleaseDate,
			SubmittedBy: r.SubmittedBy,
		}
	}
	return out, nil
}

func (s *svc) GetFilters(ctx context.Context, platform string) (*Filters, error) {
	domains, err := s.q.GetDistinctDomains(ctx, platform)
	if err != nil {
		return nil, fmt.Errorf("get domains: %w", err)
	}
	impacts, err := s.q.GetDistinctImpacts(ctx, platform)
	if err != nil {
		return nil, fmt.Errorf("get impacts: %w", err)
	}
	if domains == nil {
		domains = []string{}
	}
	if impacts == nil {
		impacts = []string{}
	}
	return &Filters{Domains: domains, Impacts: impacts}, nil
}

func (s *svc) GetVersions(ctx context.Context, platform string) ([]VersionInfo, error) {
	rows, err := s.q.GetVersionsByPlatform(ctx, platform)
	if err != nil {
		return nil, fmt.Errorf("get versions: %w", err)
	}
	versions := make([]VersionInfo, len(rows))
	for i, r := range rows {
		versions[i] = VersionInfo{
			Version:     r.Version,
			FromVer:     r.FromVer,
			ReleaseDate: r.ReleaseDate,
			SubmittedBy: r.SubmittedBy,
		}
	}
	return versions, nil
}

func (s *svc) GetJirasBetweenVersions(ctx context.Context, fromVer, toVer string) ([]JiraOutput, error) {
	// Look up "to" release to determine platform
	rel, err := s.q.GetRelease(ctx, toVer)
	if err != nil {
		return nil, fmt.Errorf("get release %s: %w", toVer, err)
	}

	chgs, err := s.tm.CalcChgs(rel.Platform, toVer, fromVer)
	if err != nil {
		return nil, fmt.Errorf("calc changes: %w", err)
	}

	if len(chgs) == 0 {
		return []JiraOutput{}, nil
	}

	ids := make([]string, len(chgs))
	for i, c := range chgs {
		ids[i] = c.ID
	}

	jiras, err := s.q.GetJirasByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("get jiras by ids: %w", err)
	}

	out := make([]JiraOutput, len(jiras))
	for i, j := range jiras {
		out[i] = JiraOutput{
			ID:       j.ID,
			Title:    j.Title,
			Impact:   j.Impact,
			Domain:   j.Domain,
			Relnotes: j.Relnotes,
		}
	}
	return out, nil
}

func (s *svc) GetTreeInfo(ctx context.Context, platform string) (*TreeInfo, error) {
	dump, err := s.tm.Dump(platform)
	if err != nil {
		return nil, err
	}
	return &TreeInfo{
		Platform:  platform,
		NodeCount: dump.NodeCount,
		Root:      dump.Root,
		Nodes:     dump.Nodes,
	}, nil
}
