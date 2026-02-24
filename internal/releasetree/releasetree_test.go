package releasetree

import (
	"strings"
	"sync"
	"testing"
)

// Tree structure used in tests:
//
//           11 (Root)
//            |
//            21          changes: {1}
//          / | \
//         /  |  \
//       31   22   23     31: {2,3,4}  22: {5}  23: {10}
//      / \    |
//     /   \   24         24: {6,7}
//    /     \
//   32      33           32: {5,6,7,8}  33: {5,6,7,10}

var (
	release11 = ReleaseInput{Ver: "11", FromVer: "", Changes: []Chg{}}
	release21 = ReleaseInput{Ver: "21", FromVer: "11", Changes: []Chg{{ID: "1"}}}
	release31 = ReleaseInput{Ver: "31", FromVer: "21", Changes: []Chg{{ID: "2"}, {ID: "3"}, {ID: "4"}}}
	release22 = ReleaseInput{Ver: "22", FromVer: "21", Changes: []Chg{{ID: "5"}}}
	release32 = ReleaseInput{Ver: "32", FromVer: "31", Changes: []Chg{{ID: "5"}, {ID: "6"}, {ID: "7"}, {ID: "8"}}}
	release24 = ReleaseInput{Ver: "24", FromVer: "22", Changes: []Chg{{ID: "6"}, {ID: "7"}}}
	release33 = ReleaseInput{Ver: "33", FromVer: "31", Changes: []Chg{{ID: "5"}, {ID: "6"}, {ID: "7"}, {ID: "10"}}}
	release23 = ReleaseInput{Ver: "23", FromVer: "21", Changes: []Chg{{ID: "10"}}}
)

func buildFullTree(t *testing.T) *ReleaseTree {
	t.Helper()

	// Batch build initial tree
	tree, err := NewReleaseTree([]ReleaseInput{release11, release21, release31, release22})
	if err != nil {
		t.Fatalf("NewReleaseTree failed: %v", err)
	}

	// Insert remaining nodes concurrently
	nodesToInsert := []ReleaseInput{release32, release24, release33, release23}
	var wg sync.WaitGroup
	for _, input := range nodesToInsert {
		wg.Add(1)
		go func(ri ReleaseInput) {
			defer wg.Done()
			if err := tree.InsertNode(ri); err != nil {
				t.Errorf("InsertNode(%s) failed: %v", ri.Ver, err)
			}
		}(input)
	}
	wg.Wait()
	return tree
}

func TestNewReleaseTree(t *testing.T) {
	tree, err := NewReleaseTree([]ReleaseInput{release11, release21, release31, release22})
	if err != nil {
		t.Fatalf("NewReleaseTree failed: %v", err)
	}
	if tree.root == nil || tree.root.version != "11" {
		t.Fatalf("expected root=11, got %v", tree.root)
	}
	if len(tree.nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(tree.nodes))
	}
}

func TestNewReleaseTree_DuplicateVersion(t *testing.T) {
	_, err := NewReleaseTree([]ReleaseInput{release11, release11})
	if err == nil {
		t.Fatal("expected error for duplicate version")
	}
	if !strings.Contains(err.Error(), "duplicate version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewReleaseTree_MissingParent(t *testing.T) {
	_, err := NewReleaseTree([]ReleaseInput{
		{Ver: "a", FromVer: "missing", Changes: nil},
	})
	if err == nil {
		t.Fatal("expected error for missing parent")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInsertNode_ConcurrentInserts(t *testing.T) {
	tree := buildFullTree(t)
	if len(tree.nodes) != 8 {
		t.Fatalf("expected 8 nodes after full build, got %d", len(tree.nodes))
	}
}

func TestInsertNode_DuplicateVersion(t *testing.T) {
	tree, _ := NewReleaseTree([]ReleaseInput{release11})
	err := tree.InsertNode(release11)
	if err == nil {
		t.Fatal("expected error for duplicate insert")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInsertNode_MissingParent(t *testing.T) {
	tree, _ := NewReleaseTree([]ReleaseInput{release11})
	err := tree.InsertNode(ReleaseInput{Ver: "x", FromVer: "missing"})
	if err == nil {
		t.Fatal("expected error for missing parent")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFindLCA(t *testing.T) {
	tree := buildFullTree(t)

	tests := []struct {
		v1, v2, expected string
	}{
		{"32", "24", "21"},
		{"33", "23", "21"},
		{"31", "22", "21"},
	}

	for _, tc := range tests {
		lca, err := tree.FindLCA(tc.v1, tc.v2)
		if err != nil {
			t.Errorf("FindLCA(%s, %s) error: %v", tc.v1, tc.v2, err)
			continue
		}
		if lca != tc.expected {
			t.Errorf("FindLCA(%s, %s) = %s, want %s", tc.v1, tc.v2, lca, tc.expected)
		}
	}
}

func TestFindLCA_NonExistentVersion(t *testing.T) {
	tree := buildFullTree(t)
	_, err := tree.FindLCA("32", "99")
	if err == nil {
		t.Fatal("expected error for non-existent version")
	}
	if !strings.Contains(err.Error(), "99") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func chgIDs(chgs []Chg) []string {
	ids := make([]string, len(chgs))
	for i, c := range chgs {
		ids[i] = c.ID
	}
	return ids
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCalcChgs(t *testing.T) {
	tree := buildFullTree(t)

	tests := []struct {
		name         string
		end, start   string
		expectedIDs  []string
		expectError  bool
		errContains  string
	}{
		{
			name:        "TC1: 32 vs 24",
			end:         "32",
			start:       "24",
			expectedIDs: []string{"2", "3", "4", "8"},
		},
		{
			name:        "TC2: Subset Fail (31 vs 22)",
			end:         "31",
			start:       "22",
			expectError: true,
			errContains: "change ID '5'",
		},
		{
			name:        "TC3: 33 vs 23",
			end:         "33",
			start:       "23",
			expectedIDs: []string{"2", "3", "4", "5", "6", "7"},
		},
		{
			name:        "TC4: Non-existent node (32 vs 99)",
			end:         "32",
			start:       "99",
			expectError: true,
			errContains: "version '99' not found",
		},
		{
			name:        "TC5: Ancestor (33 vs 21)",
			end:         "33",
			start:       "21",
			expectedIDs: []string{"2", "3", "4", "5", "6", "7", "10"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tree.CalcChgs(tc.end, tc.start)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("expected error containing %q, got: %v", tc.errContains, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := chgIDs(result)
			if !equalStringSlices(got, tc.expectedIDs) {
				t.Fatalf("got %v, want %v", got, tc.expectedIDs)
			}
		})
	}
}

func TestDump(t *testing.T) {
	tree := buildFullTree(t)
	dump := tree.Dump()
	if dump.NodeCount != 8 {
		t.Fatalf("expected 8 nodes, got %d", dump.NodeCount)
	}
	if dump.Root != "11" {
		t.Fatalf("expected root=11, got %s", dump.Root)
	}
	if len(dump.Nodes) != 8 {
		t.Fatalf("expected 8 node infos, got %d", len(dump.Nodes))
	}
}
