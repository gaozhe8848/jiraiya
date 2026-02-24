package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"jiraiya/internal/handler"
	"jiraiya/internal/service"
	"jiraiya/sql/schema"
)

// testEnv holds the shared test infrastructure.
type testEnv struct {
	srv  *httptest.Server
	pool *pgxpool.Pool
}

func setup(t *testing.T) *testEnv {
	t.Helper()
	ctx := context.Background()

	pgc, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { pgc.Terminate(context.Background()) })

	connStr, err := pgc.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	if _, err := pool.Exec(ctx, schema.InitSQL); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	if _, err := pool.Exec(ctx, schema.LtreeSQL); err != nil {
		t.Fatalf("apply ltree migration: %v", err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.New(pool, log)

	h := handler.New(svc, log)
	srv := httptest.NewServer(h.Routes())
	t.Cleanup(func() { srv.Close() })

	return &testEnv{srv: srv, pool: pool}
}

// helpers

func (e *testEnv) get(t *testing.T, path string) (int, []byte) {
	t.Helper()
	resp, err := http.Get(e.srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

func (e *testEnv) put(t *testing.T, path string, payload any) (int, []byte) {
	t.Helper()
	data, _ := json.Marshal(payload)
	resp, err := http.NewRequest(http.MethodPut, e.srv.URL+path, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("create PUT %s: %v", path, err)
	}
	resp.Header.Set("Content-Type", "application/json")
	r, err := http.DefaultClient.Do(resp)
	if err != nil {
		t.Fatalf("PUT %s: %v", path, err)
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)
	return r.StatusCode, body
}

func (e *testEnv) delete(t *testing.T, path string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, e.srv.URL+path, nil)
	if err != nil {
		t.Fatalf("create DELETE %s: %v", path, err)
	}
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	defer r.Body.Close()
	body, _ := io.ReadAll(r.Body)
	return r.StatusCode, body
}

func decode[T any](t *testing.T, data []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("decode json: %v\nbody: %s", err, data)
	}
	return v
}

// --- Tests ---

func TestEmptyDatabase(t *testing.T) {
	env := setup(t)

	t.Run("GET releases returns empty array", func(t *testing.T) {
		code, body := env.get(t, "/api/releases?platform=ios")
		if code != 200 {
			t.Fatalf("expected 200, got %d: %s", code, body)
		}
		got := decode[[]any](t, body)
		if len(got) != 0 {
			t.Fatalf("expected empty array, got %v", got)
		}
	})

	t.Run("GET filters returns empty lists", func(t *testing.T) {
		code, body := env.get(t, "/api/filters?platform=ios")
		if code != 200 {
			t.Fatalf("expected 200, got %d: %s", code, body)
		}
		got := decode[map[string][]string](t, body)
		if len(got["domains"]) != 0 || len(got["impacts"]) != 0 {
			t.Fatalf("expected empty filters, got %v", got)
		}
	})

	t.Run("GET versions returns empty array", func(t *testing.T) {
		code, body := env.get(t, "/api/versions?platform=ios")
		if code != 200 {
			t.Fatalf("expected 200, got %d: %s", code, body)
		}
		got := decode[[]any](t, body)
		if len(got) != 0 {
			t.Fatalf("expected empty array, got %v", got)
		}
	})
}

func TestMissingQueryParams(t *testing.T) {
	env := setup(t)

	tests := []struct {
		name string
		path string
	}{
		{"releases missing platform and version", "/api/releases"},
		{"filters missing platform", "/api/filters"},
		{"versions missing platform", "/api/versions"},
		{"jiras missing from and to", "/api/jiras"},
		{"jiras missing to", "/api/jiras?from=1.0.0"},
		{"tree missing platform", "/api/admin/tree"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, body := env.get(t, tc.path)
			if code != 400 {
				t.Fatalf("expected 400, got %d: %s", code, body)
			}
			got := decode[map[string]string](t, body)
			if got["error"] == "" {
				t.Fatalf("expected error message, got %v", got)
			}
		})
	}
}

func TestSubmitValidation(t *testing.T) {
	env := setup(t)

	t.Run("missing version", func(t *testing.T) {
		code, body := env.put(t, "/api/releases", map[string]any{
			"release": map[string]string{"version": "", "platform": "ios"},
			"changes": []any{},
		})
		if code != 400 {
			t.Fatalf("expected 400, got %d: %s", code, body)
		}
		got := decode[map[string]any](t, body)
		if got["error"] != "validation failed" {
			t.Fatalf("expected validation failed, got %v", got)
		}
	})

	t.Run("missing platform", func(t *testing.T) {
		code, body := env.put(t, "/api/releases", map[string]any{
			"release": map[string]string{"version": "1.0.0", "platform": ""},
			"changes": []any{},
		})
		if code != 400 {
			t.Fatalf("expected 400, got %d: %s", code, body)
		}
	})

	t.Run("missing jira id", func(t *testing.T) {
		code, body := env.put(t, "/api/releases", map[string]any{
			"release": map[string]string{"version": "1.0.0", "platform": "ios"},
			"changes": []map[string]string{{"id": "", "title": "bad"}},
		})
		if code != 400 {
			t.Fatalf("expected 400, got %d: %s", code, body)
		}
		got := decode[map[string]any](t, body)
		details := got["details"].([]any)
		if len(details) != 1 {
			t.Fatalf("expected 1 detail, got %d", len(details))
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		resp, err := http.NewRequest(http.MethodPut, env.srv.URL+"/api/releases",
			bytes.NewReader([]byte("not json")))
		if err != nil {
			t.Fatal(err)
		}
		resp.Header.Set("Content-Type", "application/json")
		r, err := http.DefaultClient.Do(resp)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		if r.StatusCode != 400 {
			t.Fatalf("expected 400, got %d", r.StatusCode)
		}
	})
}

func TestFullLifecycle(t *testing.T) {
	env := setup(t)

	// Submit first release (root)
	code, body := env.put(t, "/api/releases", map[string]any{
		"release": map[string]string{
			"version":      "1.0.0",
			"from_ver":     "",
			"platform":     "ios",
			"release_date": "2026-01-01",
			"submitted_by": "alice",
		},
		"changes": []map[string]string{
			{"id": "JIRA-1", "title": "Login feature", "domain": "auth", "impact": "high", "relnotes": "Added login"},
			{"id": "JIRA-2", "title": "Signup flow", "domain": "auth", "impact": "medium", "relnotes": "Added signup"},
		},
	})
	if code != 200 {
		t.Fatalf("submit release 1.0.0: expected 200, got %d: %s", code, body)
	}

	// Submit second release (child of 1.0.0)
	code, body = env.put(t, "/api/releases", map[string]any{
		"release": map[string]string{
			"version":      "1.1.0",
			"from_ver":     "1.0.0",
			"platform":     "ios",
			"release_date": "2026-02-01",
			"submitted_by": "bob",
		},
		"changes": []map[string]string{
			{"id": "JIRA-3", "title": "Dark mode", "domain": "ui", "impact": "low", "relnotes": "Added dark mode"},
		},
	})
	if code != 200 {
		t.Fatalf("submit release 1.1.0: expected 200, got %d: %s", code, body)
	}

	// Submit third release (another child of 1.0.0, different branch)
	code, body = env.put(t, "/api/releases", map[string]any{
		"release": map[string]string{
			"version":      "1.0.1",
			"from_ver":     "1.0.0",
			"platform":     "ios",
			"release_date": "2026-01-15",
			"submitted_by": "charlie",
		},
		"changes": []map[string]string{
			{"id": "JIRA-4", "title": "Hotfix crash", "domain": "core", "impact": "critical", "relnotes": "Fixed crash"},
		},
	})
	if code != 200 {
		t.Fatalf("submit release 1.0.1: expected 200, got %d: %s", code, body)
	}

	// Verify GET releases by platform
	t.Run("get all releases by platform", func(t *testing.T) {
		code, body := env.get(t, "/api/releases?platform=ios")
		if code != 200 {
			t.Fatalf("expected 200, got %d: %s", code, body)
		}
		releases := decode[[]map[string]string](t, body)
		if len(releases) != 3 {
			t.Fatalf("expected 3 releases, got %d", len(releases))
		}
	})

	// Verify GET single release by version
	t.Run("get single release", func(t *testing.T) {
		code, body := env.get(t, "/api/releases?version=1.0.0")
		if code != 200 {
			t.Fatalf("expected 200, got %d: %s", code, body)
		}
		releases := decode[[]map[string]string](t, body)
		if len(releases) != 1 {
			t.Fatalf("expected 1 release, got %d", len(releases))
		}
		if releases[0]["version"] != "1.0.0" {
			t.Fatalf("expected version 1.0.0, got %s", releases[0]["version"])
		}
		if releases[0]["submitted_by"] != "alice" {
			t.Fatalf("expected submitted_by alice, got %s", releases[0]["submitted_by"])
		}
	})

	// Verify filters
	t.Run("get filters", func(t *testing.T) {
		code, body := env.get(t, "/api/filters?platform=ios")
		if code != 200 {
			t.Fatalf("expected 200, got %d: %s", code, body)
		}
		filters := decode[map[string][]string](t, body)
		if len(filters["domains"]) != 3 { // auth, core, ui
			t.Fatalf("expected 3 domains, got %v", filters["domains"])
		}
		if len(filters["impacts"]) != 4 { // critical, high, low, medium
			t.Fatalf("expected 4 impacts, got %v", filters["impacts"])
		}
	})

	// Verify versions
	t.Run("get versions", func(t *testing.T) {
		code, body := env.get(t, "/api/versions?platform=ios")
		if code != 200 {
			t.Fatalf("expected 200, got %d: %s", code, body)
		}
		versions := decode[[]map[string]string](t, body)
		if len(versions) != 3 {
			t.Fatalf("expected 3 versions, got %d", len(versions))
		}
	})

	// Verify jiras between versions
	t.Run("jiras from 1.0.0 to 1.1.0", func(t *testing.T) {
		code, body := env.get(t, "/api/jiras?from=1.0.0&to=1.1.0")
		if code != 200 {
			t.Fatalf("expected 200, got %d: %s", code, body)
		}
		jiras := decode[[]map[string]string](t, body)
		if len(jiras) != 1 {
			t.Fatalf("expected 1 jira, got %d: %s", len(jiras), body)
		}
		if jiras[0]["id"] != "JIRA-3" {
			t.Fatalf("expected JIRA-3, got %s", jiras[0]["id"])
		}
	})

	t.Run("jiras from 1.0.0 to 1.0.1", func(t *testing.T) {
		code, body := env.get(t, "/api/jiras?from=1.0.0&to=1.0.1")
		if code != 200 {
			t.Fatalf("expected 200, got %d: %s", code, body)
		}
		jiras := decode[[]map[string]string](t, body)
		if len(jiras) != 1 {
			t.Fatalf("expected 1 jira, got %d: %s", len(jiras), body)
		}
		if jiras[0]["id"] != "JIRA-4" {
			t.Fatalf("expected JIRA-4, got %s", jiras[0]["id"])
		}
	})

	// Verify tree
	t.Run("get tree", func(t *testing.T) {
		code, body := env.get(t, "/api/admin/tree?platform=ios")
		if code != 200 {
			t.Fatalf("expected 200, got %d: %s", code, body)
		}
		tree := decode[map[string]any](t, body)
		if tree["platform"] != "ios" {
			t.Fatalf("expected platform ios, got %v", tree["platform"])
		}
		nodeCount := int(tree["node_count"].(float64))
		if nodeCount != 3 {
			t.Fatalf("expected 3 nodes, got %d", nodeCount)
		}
		if tree["root"] != "1.0.0" {
			t.Fatalf("expected root 1.0.0, got %v", tree["root"])
		}
	})

	// Delete a release and verify
	t.Run("delete release", func(t *testing.T) {
		code, body := env.delete(t, "/api/releases/1.1.0")
		if code != 200 {
			t.Fatalf("expected 200, got %d: %s", code, body)
		}

		// Verify it's gone
		code, body = env.get(t, "/api/releases?platform=ios")
		if code != 200 {
			t.Fatalf("expected 200, got %d: %s", code, body)
		}
		releases := decode[[]map[string]string](t, body)
		if len(releases) != 2 {
			t.Fatalf("expected 2 releases after delete, got %d", len(releases))
		}
		for _, r := range releases {
			if r["version"] == "1.1.0" {
				t.Fatal("1.1.0 should have been deleted")
			}
		}

		// Verify tree rebuilt
		code, body = env.get(t, "/api/admin/tree?platform=ios")
		if code != 200 {
			t.Fatalf("expected 200, got %d: %s", code, body)
		}
		tree := decode[map[string]any](t, body)
		nodeCount := int(tree["node_count"].(float64))
		if nodeCount != 2 {
			t.Fatalf("expected 2 nodes after delete, got %d", nodeCount)
		}
	})
}

func TestUpsertRelease(t *testing.T) {
	env := setup(t)

	// Submit initial release
	env.put(t, "/api/releases", map[string]any{
		"release": map[string]string{
			"version": "2.0.0", "from_ver": "", "platform": "android",
			"release_date": "2026-01-01", "submitted_by": "alice",
		},
		"changes": []map[string]string{
			{"id": "A-1", "title": "Feature A", "domain": "core", "impact": "high", "relnotes": "Added A"},
		},
	})

	// Upsert same version with different jiras
	code, body := env.put(t, "/api/releases", map[string]any{
		"release": map[string]string{
			"version": "2.0.0", "from_ver": "", "platform": "android",
			"release_date": "2026-01-02", "submitted_by": "bob",
		},
		"changes": []map[string]string{
			{"id": "A-1", "title": "Feature A updated", "domain": "core", "impact": "high", "relnotes": "Updated A"},
			{"id": "A-2", "title": "Feature B", "domain": "ui", "impact": "low", "relnotes": "Added B"},
		},
	})
	if code != 200 {
		t.Fatalf("upsert: expected 200, got %d: %s", code, body)
	}

	// Verify release was updated
	code, body = env.get(t, "/api/releases?version=2.0.0")
	if code != 200 {
		t.Fatalf("expected 200, got %d: %s", code, body)
	}
	releases := decode[[]map[string]string](t, body)
	if releases[0]["submitted_by"] != "bob" {
		t.Fatalf("expected submitted_by bob after upsert, got %s", releases[0]["submitted_by"])
	}
	if releases[0]["release_date"] != "2026-01-02" {
		t.Fatalf("expected release_date 2026-01-02, got %s", releases[0]["release_date"])
	}

	// Verify filters reflect both jiras
	code, body = env.get(t, "/api/filters?platform=android")
	if code != 200 {
		t.Fatalf("expected 200, got %d: %s", code, body)
	}
	filters := decode[map[string][]string](t, body)
	if len(filters["domains"]) != 2 {
		t.Fatalf("expected 2 domains after upsert, got %v", filters["domains"])
	}
}

func TestMultiplePlatforms(t *testing.T) {
	env := setup(t)

	// Submit ios release (versions are globally unique, so use distinct names)
	env.put(t, "/api/releases", map[string]any{
		"release": map[string]string{
			"version": "ios-1.0.0", "from_ver": "", "platform": "ios",
			"release_date": "2026-01-01", "submitted_by": "alice",
		},
		"changes": []map[string]string{
			{"id": "I-1", "title": "iOS feature", "domain": "mobile", "impact": "high", "relnotes": "iOS stuff"},
		},
	})

	// Submit android release
	env.put(t, "/api/releases", map[string]any{
		"release": map[string]string{
			"version": "android-1.0.0", "from_ver": "", "platform": "android",
			"release_date": "2026-01-01", "submitted_by": "bob",
		},
		"changes": []map[string]string{
			{"id": "A-1", "title": "Android feature", "domain": "mobile", "impact": "low", "relnotes": "Android stuff"},
		},
	})

	// ios releases should only contain ios
	code, body := env.get(t, "/api/releases?platform=ios")
	if code != 200 {
		t.Fatalf("expected 200, got %d: %s", code, body)
	}
	releases := decode[[]map[string]string](t, body)
	if len(releases) != 1 {
		t.Fatalf("expected 1 ios release, got %d", len(releases))
	}
	if releases[0]["platform"] != "ios" {
		t.Fatalf("expected ios, got %s", releases[0]["platform"])
	}

	// android releases should only contain android
	code, body = env.get(t, "/api/releases?platform=android")
	if code != 200 {
		t.Fatalf("expected 200, got %d: %s", code, body)
	}
	releases = decode[[]map[string]string](t, body)
	if len(releases) != 1 {
		t.Fatalf("expected 1 android release, got %d", len(releases))
	}

	// Each platform should have its own tree
	code, body = env.get(t, "/api/admin/tree?platform=ios")
	if code != 200 {
		t.Fatalf("expected 200, got %d: %s", code, body)
	}
	tree := decode[map[string]any](t, body)
	if int(tree["node_count"].(float64)) != 1 {
		t.Fatalf("expected 1 ios node, got %v", tree["node_count"])
	}

	code, body = env.get(t, "/api/admin/tree?platform=android")
	if code != 200 {
		t.Fatalf("expected 200, got %d: %s", code, body)
	}
	tree = decode[map[string]any](t, body)
	if int(tree["node_count"].(float64)) != 1 {
		t.Fatalf("expected 1 android node, got %v", tree["node_count"])
	}
}

func TestDeleteLastRelease(t *testing.T) {
	env := setup(t)

	// Submit a single release
	env.put(t, "/api/releases", map[string]any{
		"release": map[string]string{
			"version": "1.0.0", "from_ver": "", "platform": "web",
			"release_date": "2026-01-01", "submitted_by": "alice",
		},
		"changes": []map[string]string{
			{"id": "W-1", "title": "Web feature", "domain": "frontend", "impact": "high", "relnotes": "Web stuff"},
		},
	})

	// Delete it
	code, body := env.delete(t, "/api/releases/1.0.0")
	if code != 200 {
		t.Fatalf("expected 200, got %d: %s", code, body)
	}

	// Platform should have no releases
	code, body = env.get(t, "/api/releases?platform=web")
	if code != 200 {
		t.Fatalf("expected 200, got %d: %s", code, body)
	}
	releases := decode[[]any](t, body)
	if len(releases) != 0 {
		t.Fatalf("expected 0 releases, got %d", len(releases))
	}

	// Tree should be gone (returns 500 since no releases exist)
	code, _ = env.get(t, "/api/admin/tree?platform=web")
	if code != 500 {
		t.Fatalf("expected 500 for deleted tree, got %d", code)
	}
}
