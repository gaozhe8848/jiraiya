package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"jiraiya/internal/db"
	"jiraiya/internal/releasetree"
)

// TreeManager holds one in-memory ReleaseTree per platform.
type TreeManager struct {
	mu    sync.RWMutex
	trees map[string]*releasetree.ReleaseTree
	log   *slog.Logger
}

// NewTreeManager creates an empty TreeManager.
func NewTreeManager(log *slog.Logger) *TreeManager {
	return &TreeManager{
		trees: make(map[string]*releasetree.ReleaseTree),
		log:   log,
	}
}

// LoadAll queries all platforms from the DB and builds each tree.
func (tm *TreeManager) LoadAll(ctx context.Context, q *db.Queries) error {
	platforms, err := q.GetAllPlatforms(ctx)
	if err != nil {
		return fmt.Errorf("TreeManager.LoadAll: get platforms: %w", err)
	}

	for _, platform := range platforms {
		if err := tm.buildTree(ctx, q, platform); err != nil {
			return fmt.Errorf("TreeManager.LoadAll: build tree for %s: %w", platform, err)
		}
	}
	return nil
}

// buildTree builds a tree for a single platform from DB data.
func (tm *TreeManager) buildTree(ctx context.Context, q *db.Queries, platform string) error {
	releases, err := q.GetAllReleasesByPlatform(ctx, platform)
	if err != nil {
		return err
	}

	inputs := make([]releasetree.ReleaseInput, 0, len(releases))
	for _, r := range releases {
		jiraIDs, err := q.GetJiraIDsByRelease(ctx, r.Version)
		if err != nil {
			return fmt.Errorf("get jiras for %s: %w", r.Version, err)
		}
		chgs := make([]releasetree.Chg, len(jiraIDs))
		for i, id := range jiraIDs {
			chgs[i] = releasetree.Chg{ID: id}
		}
		inputs = append(inputs, releasetree.ReleaseInput{
			Ver:     r.Version,
			FromVer: r.FromVer,
			Changes: chgs,
		})
	}

	tree, err := releasetree.NewReleaseTree(inputs)
	if err != nil {
		return err
	}

	tm.mu.Lock()
	tm.trees[platform] = tree
	tm.mu.Unlock()

	dump := tree.Dump()
	tm.log.Info("tree built", "platform", platform, "node_count", dump.NodeCount, "root", dump.Root)
	return nil
}

// Insert adds a node to the platform tree, creating the tree if needed.
func (tm *TreeManager) Insert(platform string, input releasetree.ReleaseInput) error {
	tm.mu.Lock()
	tree, exists := tm.trees[platform]
	if !exists {
		// First release for this platform — create a new tree
		t, err := releasetree.NewReleaseTree([]releasetree.ReleaseInput{input})
		if err != nil {
			tm.mu.Unlock()
			return err
		}
		tm.trees[platform] = t
		tm.mu.Unlock()

		dump := t.Dump()
		tm.log.Info("tree created", "platform", platform, "node_count", dump.NodeCount, "root", dump.Root, "inserted_version", input.Ver)
		return nil
	}
	tm.mu.Unlock()

	if err := tree.InsertNode(input); err != nil {
		return err
	}

	dump := tree.Dump()
	tm.log.Info("tree updated", "platform", platform, "node_count", dump.NodeCount, "root", dump.Root, "inserted_version", input.Ver)
	return nil
}

// Rebuild rebuilds a platform tree from DB after a delete.
func (tm *TreeManager) Rebuild(ctx context.Context, q *db.Queries, platform string) error {
	// Check if there are any releases left for this platform
	releases, err := q.GetAllReleasesByPlatform(ctx, platform)
	if err != nil {
		return err
	}
	if len(releases) == 0 {
		tm.mu.Lock()
		delete(tm.trees, platform)
		tm.mu.Unlock()
		tm.log.Info("tree removed", "platform", platform)
		return nil
	}
	return tm.buildTree(ctx, q, platform)
}

// CalcChgs delegates to the platform tree's CalcChgs.
func (tm *TreeManager) CalcChgs(platform, endVer, startVer string) ([]releasetree.Chg, error) {
	tm.mu.RLock()
	tree, exists := tm.trees[platform]
	tm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no tree for platform %q", platform)
	}
	return tree.CalcChgs(endVer, startVer)
}

// Dump returns the tree dump for a platform.
func (tm *TreeManager) Dump(platform string) (*releasetree.TreeDump, error) {
	tm.mu.RLock()
	tree, exists := tm.trees[platform]
	tm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no tree for platform %q", platform)
	}
	d := tree.Dump()
	return &d, nil
}
