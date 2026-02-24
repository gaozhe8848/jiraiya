package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"jiraiya/internal/db"
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
	// Validate both versions exist and belong to the same platform.
	// The SQL silently returns wrong results for missing versions (NULL paths)
	// or cross-platform queries (empty LCA).
	toRel, err := s.q.GetRelease(ctx, toVer)
	if err != nil {
		return nil, fmt.Errorf("version %q not found: %w", toVer, err)
	}
	fromRel, err := s.q.GetRelease(ctx, fromVer)
	if err != nil {
		return nil, fmt.Errorf("version %q not found: %w", fromVer, err)
	}
	if toRel.Platform != fromRel.Platform {
		return nil, fmt.Errorf("versions are on different platforms: %q (%s) vs %q (%s)", toVer, toRel.Platform, fromVer, fromRel.Platform)
	}

	// Verify the ancestor chain is intact for both versions and that
	// they share a common root. Catches:
	// - deleted intermediate releases (stale ltree paths, incomplete results)
	// - disconnected subtrees on the same platform (empty LCA, wrong results)
	var paths [2]string
	for i, ver := range []string{toVer, fromVer} {
		path, err := s.q.GetReleasePath(ctx, ver)
		if err != nil {
			return nil, fmt.Errorf("get path for %q: %w", ver, err)
		}
		paths[i] = path
		expectedDepth := int64(strings.Count(path, ".") + 1)
		actualCount, err := s.q.CountPathAncestors(ctx, path)
		if err != nil {
			return nil, fmt.Errorf("check path integrity for %q: %w", ver, err)
		}
		if actualCount != expectedDepth {
			return nil, fmt.Errorf("broken release chain: version %q expects %d ancestors but only %d exist", ver, expectedDepth, actualCount)
		}
	}
	if strings.SplitN(paths[0], ".", 2)[0] != strings.SplitN(paths[1], ".", 2)[0] {
		return nil, fmt.Errorf("versions %q and %q do not share a common ancestor", toVer, fromVer)
	}

	ids, err := s.q.CalcChgs(ctx, db.CalcChgsParams{
		ToVersion:   toVer,
		FromVersion: fromVer,
	})
	if err != nil {
		return nil, fmt.Errorf("calc changes: %w", err)
	}

	if len(ids) == 0 {
		return []JiraOutput{}, nil
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
	releases, err := s.q.GetAllReleasesByPlatform(ctx, platform)
	if err != nil {
		return nil, fmt.Errorf("get releases: %w", err)
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases for platform %q", platform)
	}

	// Build parent->children map and find root
	childrenMap := make(map[string][]string)
	fromVerMap := make(map[string]string)
	var root string
	for _, r := range releases {
		fromVerMap[r.Version] = r.FromVer
		if r.FromVer == "" {
			root = r.Version
		} else {
			childrenMap[r.FromVer] = append(childrenMap[r.FromVer], r.Version)
		}
	}

	// Sort children for deterministic output
	for k := range childrenMap {
		sort.Strings(childrenMap[k])
	}

	// Build node infos sorted by version
	versionList := make([]string, len(releases))
	for i, r := range releases {
		versionList[i] = r.Version
	}
	sort.Strings(versionList)

	nodes := make([]NodeInfo, 0, len(releases))
	for _, v := range versionList {
		jiraIDs, err := s.q.GetJiraIDsByRelease(ctx, v)
		if err != nil {
			return nil, fmt.Errorf("get jiras for %s: %w", v, err)
		}
		if jiraIDs == nil {
			jiraIDs = []string{}
		}
		children := childrenMap[v]
		if children == nil {
			children = []string{}
		}
		nodes = append(nodes, NodeInfo{
			Version:  v,
			FromVer:  fromVerMap[v],
			Changes:  jiraIDs,
			Children: children,
		})
	}

	return &TreeInfo{
		Platform:  platform,
		NodeCount: len(releases),
		Root:      root,
		Nodes:     nodes,
	}, nil
}
