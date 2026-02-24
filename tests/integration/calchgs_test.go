package integration

import (
	"sort"
	"testing"
)

// Tree structure (matches the old releasetree_test.go):
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

type releaseData struct {
	version  string
	fromVer  string
	jiraIDs  []string
}

var treeReleases = []releaseData{
	{"11", "", nil},
	{"21", "11", []string{"1"}},
	{"31", "21", []string{"2", "3", "4"}},
	{"22", "21", []string{"5"}},
	{"23", "21", []string{"10"}},
	{"32", "31", []string{"5", "6", "7", "8"}},
	{"24", "22", []string{"6", "7"}},
	{"33", "31", []string{"5", "6", "7", "10"}},
}

func setupCalcChgsTree(t *testing.T) *testEnv {
	t.Helper()
	env := setup(t)

	for _, r := range treeReleases {
		changes := make([]map[string]string, len(r.jiraIDs))
		for i, id := range r.jiraIDs {
			changes[i] = map[string]string{
				"id": id, "title": "jira " + id, "domain": "d", "impact": "low", "relnotes": "",
			}
		}
		code, body := env.put(t, "/api/releases", map[string]any{
			"release": map[string]string{
				"version":      r.version,
				"from_ver":     r.fromVer,
				"platform":     "test",
				"release_date": "2026-01-01",
				"submitted_by": "tester",
			},
			"changes": changes,
		})
		if code != 200 {
			t.Fatalf("submit release %s: expected 200, got %d: %s", r.version, code, body)
		}
	}

	return env
}

func TestCalcChgs(t *testing.T) {
	env := setupCalcChgsTree(t)

	tests := []struct {
		name        string
		to          string
		from        string
		expectedIDs []string
		expectError bool
	}{
		{
			name:        "TC1: 32 vs 24",
			to:          "32",
			from:        "24",
			expectedIDs: []string{"2", "3", "4", "8"},
		},
		{
			name:        "TC2: 31 vs 22 (was error, now returns result)",
			to:          "31",
			from:        "22",
			expectedIDs: []string{"2", "3", "4"},
		},
		{
			name:        "TC3: 33 vs 23",
			to:          "33",
			from:        "23",
			expectedIDs: []string{"2", "3", "4", "5", "6", "7"},
		},
		{
			name:        "TC4: Non-existent version (32 vs 99)",
			to:          "32",
			from:        "99",
			expectError: true,
		},
		{
			name:        "TC5: Ancestor (33 vs 21)",
			to:          "33",
			from:        "21",
			expectedIDs: []string{"2", "3", "4", "5", "6", "7", "10"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, body := env.get(t, "/api/jiras?from="+tc.from+"&to="+tc.to)
			if tc.expectError {
				if code == 200 {
					t.Fatalf("expected error, got 200: %s", body)
				}
				return
			}
			if code != 200 {
				t.Fatalf("expected 200, got %d: %s", code, body)
			}

			jiras := decode[[]map[string]string](t, body)
			gotIDs := make([]string, len(jiras))
			for i, j := range jiras {
				gotIDs[i] = j["id"]
			}
			sort.Strings(gotIDs)

			expected := make([]string, len(tc.expectedIDs))
			copy(expected, tc.expectedIDs)
			sort.Strings(expected)

			if len(gotIDs) != len(expected) {
				t.Fatalf("expected %v, got %v", expected, gotIDs)
			}
			for i := range expected {
				if gotIDs[i] != expected[i] {
					t.Fatalf("expected %v, got %v", expected, gotIDs)
				}
			}
		})
	}
}

func TestCalcChgsCrossPlatform(t *testing.T) {
	env := setup(t)

	// Create a release on platform "alpha"
	code, body := env.put(t, "/api/releases", map[string]any{
		"release": map[string]string{
			"version": "a-1", "from_ver": "", "platform": "alpha",
			"release_date": "2026-01-01", "submitted_by": "tester",
		},
		"changes": []map[string]string{
			{"id": "X-1", "title": "t", "domain": "d", "impact": "low", "relnotes": ""},
		},
	})
	if code != 200 {
		t.Fatalf("submit a-1: expected 200, got %d: %s", code, body)
	}

	// Create a release on platform "beta"
	code, body = env.put(t, "/api/releases", map[string]any{
		"release": map[string]string{
			"version": "b-1", "from_ver": "", "platform": "beta",
			"release_date": "2026-01-01", "submitted_by": "tester",
		},
		"changes": []map[string]string{
			{"id": "X-2", "title": "t", "domain": "d", "impact": "low", "relnotes": ""},
		},
	})
	if code != 200 {
		t.Fatalf("submit b-1: expected 200, got %d: %s", code, body)
	}

	// Query across platforms should error
	code, _ = env.get(t, "/api/jiras?from=a-1&to=b-1")
	if code == 200 {
		t.Fatal("expected error for cross-platform query, got 200")
	}
}

func TestCalcChgsDisconnectedTree(t *testing.T) {
	env := setup(t)

	// Build a chain: root -> middle -> leaf
	for _, r := range []releaseData{
		{"root", "", nil},
		{"middle", "root", []string{"J-1"}},
		{"leaf", "middle", []string{"J-2"}},
	} {
		changes := make([]map[string]string, len(r.jiraIDs))
		for i, id := range r.jiraIDs {
			changes[i] = map[string]string{
				"id": id, "title": "jira " + id, "domain": "d", "impact": "low", "relnotes": "",
			}
		}
		code, body := env.put(t, "/api/releases", map[string]any{
			"release": map[string]string{
				"version": r.version, "from_ver": r.fromVer, "platform": "p",
				"release_date": "2026-01-01", "submitted_by": "tester",
			},
			"changes": changes,
		})
		if code != 200 {
			t.Fatalf("submit %s: expected 200, got %d: %s", r.version, code, body)
		}
	}

	// Create a second disconnected root on the same platform
	code, body := env.put(t, "/api/releases", map[string]any{
		"release": map[string]string{
			"version": "orphan", "from_ver": "", "platform": "p",
			"release_date": "2026-01-01", "submitted_by": "tester",
		},
		"changes": []map[string]string{
			{"id": "J-3", "title": "jira J-3", "domain": "d", "impact": "low", "relnotes": ""},
		},
	})
	if code != 200 {
		t.Fatalf("submit orphan: expected 200, got %d: %s", code, body)
	}

	// Query between disconnected subtrees should error (no common ancestor)
	code, _ = env.get(t, "/api/jiras?from=leaf&to=orphan")
	if code == 200 {
		t.Fatal("expected error for disconnected tree query, got 200")
	}

	// Query within the connected chain should still work
	code, body = env.get(t, "/api/jiras?from=root&to=leaf")
	if code != 200 {
		t.Fatalf("expected 200, got %d: %s", code, body)
	}
	jiras := decode[[]map[string]string](t, body)
	gotIDs := make([]string, len(jiras))
	for i, j := range jiras {
		gotIDs[i] = j["id"]
	}
	sort.Strings(gotIDs)
	expected := []string{"J-1", "J-2"}
	if len(gotIDs) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, gotIDs)
	}
	for i := range expected {
		if gotIDs[i] != expected[i] {
			t.Fatalf("expected %v, got %v", expected, gotIDs)
		}
	}
}

func TestCalcChgsBrokenChain(t *testing.T) {
	env := setup(t)

	// Build a chain: root -> middle -> leaf
	for _, r := range []releaseData{
		{"root", "", nil},
		{"middle", "root", []string{"J-1"}},
		{"leaf", "middle", []string{"J-2"}},
	} {
		changes := make([]map[string]string, len(r.jiraIDs))
		for i, id := range r.jiraIDs {
			changes[i] = map[string]string{
				"id": id, "title": "jira " + id, "domain": "d", "impact": "low", "relnotes": "",
			}
		}
		code, body := env.put(t, "/api/releases", map[string]any{
			"release": map[string]string{
				"version": r.version, "from_ver": r.fromVer, "platform": "p",
				"release_date": "2026-01-01", "submitted_by": "tester",
			},
			"changes": changes,
		})
		if code != 200 {
			t.Fatalf("submit %s: expected 200, got %d: %s", r.version, code, body)
		}
	}

	// Sanity check: query works before deletion
	code, body := env.get(t, "/api/jiras?from=root&to=leaf")
	if code != 200 {
		t.Fatalf("expected 200 before deletion, got %d: %s", code, body)
	}

	// Delete the middle node, breaking the chain
	code, body = env.delete(t, "/api/releases/middle")
	if code != 200 {
		t.Fatalf("delete middle: expected 200, got %d: %s", code, body)
	}

	// Query through the broken chain should error
	code, _ = env.get(t, "/api/jiras?from=root&to=leaf")
	if code == 200 {
		t.Fatal("expected error for broken chain query, got 200")
	}
}

func TestGetTreeInfo(t *testing.T) {
	env := setupCalcChgsTree(t)

	code, body := env.get(t, "/api/admin/tree?platform=test")
	if code != 200 {
		t.Fatalf("expected 200, got %d: %s", code, body)
	}

	tree := decode[map[string]any](t, body)
	if tree["platform"] != "test" {
		t.Fatalf("expected platform test, got %v", tree["platform"])
	}
	nodeCount := int(tree["node_count"].(float64))
	if nodeCount != 8 {
		t.Fatalf("expected 8 nodes, got %d", nodeCount)
	}
	if tree["root"] != "11" {
		t.Fatalf("expected root 11, got %v", tree["root"])
	}

	nodes := tree["nodes"].([]any)
	if len(nodes) != 8 {
		t.Fatalf("expected 8 node infos, got %d", len(nodes))
	}
}
