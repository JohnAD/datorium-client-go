package todo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	datorium "github.com/JohnAD/datorium-client-go/v2"
	"github.com/JohnAD/datorium-client-go/v2/shard"
)

// TestTodoFixturesParse ensures the Compose establishment tree is structurally
// valid without requiring Docker.
func TestTodoFixturesParse(t *testing.T) {
	cfgDir := filepath.Join("fixtures", "server1", ".config")
	mustJSONFile(t, filepath.Join(cfgDir, "__general.json"))
	mustJSONFile(t, filepath.Join(cfgDir, "__servers.json"))
	mustJSONFile(t, filepath.Join(cfgDir, "__shard-map.json"))
	mustJSONFile(t, filepath.Join(cfgDir, "__auth.json"))
	for _, name := range []string{
		"Users.schema.json", "Users.schema.0.json",
		"TodoLists.schema.json", "TodoLists.schema.0.json",
		"Todos.schema.json", "Todos.schema.0.json",
		"Todos.search.byStatus.json",
	} {
		mustJSONFile(t, filepath.Join(cfgDir, name))
	}

	rawShard, err := os.ReadFile(filepath.Join(cfgDir, "__shard-map.json"))
	if err != nil {
		t.Fatal(err)
	}
	var shardDoc struct {
		ShardMap struct {
			Default map[string]datorium.ShardAssignment `json:"default"`
		} `json:"shardMap"`
	}
	if err := json.Unmarshal(rawShard, &shardDoc); err != nil {
		t.Fatal(err)
	}
	var ranges []shard.Range
	for raw := range shardDoc.ShardMap.Default {
		r, err := shard.ParseRange(raw)
		if err != nil {
			t.Fatalf("range %q: %v", raw, err)
		}
		ranges = append(ranges, r)
	}
	if err := shard.ValidateFullCoverage(ranges); err != nil {
		t.Fatal(err)
	}
	if _, ok := shardDoc.ShardMap.Default["00-7F"]; !ok {
		t.Fatal("missing 00-7F")
	}
	if _, ok := shardDoc.ShardMap.Default["80-FF"]; !ok {
		t.Fatal("missing 80-FF")
	}

	if _, err := os.Stat(filepath.Join("secrets", "dev-signing-key.pem")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join("secrets", "bootstrap-secret.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("docker-compose.yml"); err != nil {
		t.Fatal(err)
	}
}

func mustJSONFile(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}
