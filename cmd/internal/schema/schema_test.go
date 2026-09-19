package schema_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/markdown-bible/holy-bible/cmd/internal/schema"
)

func TestFingerprintStable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.json")
	snap := schema.Snapshot{
		Tables: []schema.Table{
			{Name: "Book", Columns: []schema.Column{{Name: "id", Type: "TEXT"}}},
		},
	}
	if err := schema.Save(path, snap); err != nil {
		t.Fatal(err)
	}
	loaded, err := schema.LoadExpected(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Fingerprint == "" {
		t.Fatal("empty fingerprint")
	}
	if err := schema.Save(path, snap); err != nil {
		t.Fatal(err)
	}
	loaded2, err := schema.LoadExpected(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded2.Fingerprint != loaded.Fingerprint {
		t.Fatal("fingerprint not stable")
	}
}

func TestRefreshIntegration(t *testing.T) {
	dbPath := "../../../.cache/bible.db"
	if _, err := os.Stat(dbPath); err != nil {
		t.Skip("missing db")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "expected.json")
	if err := schema.Refresh(dbPath, path); err != nil {
		t.Fatal(err)
	}
	if err := schema.Check(dbPath, path); err != nil {
		t.Fatal(err)
	}
}
