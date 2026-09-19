//go:build integration

package export_test

import (
	"context"
	"os"
	"testing"

	"github.com/markdown-bible/holy-bible/cmd/internal/db"
)

func TestIntegrationChapterPresent(t *testing.T) {
	dbPath := "../../../.cache/bible.db"
	if _, err := os.Stat(dbPath); err != nil {
		t.Skip("missing .cache/bible.db")
	}
	store, err := db.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	translations, err := store.ListTranslations(ctx)
	if err != nil || len(translations) == 0 {
		t.Fatal(err)
	}
	tr := translations[0]
	for _, cand := range translations {
		if cand.ID == "BSB" {
			tr = cand
			break
		}
	}

	ch, err := store.GetChapter(ctx, tr.ID, "GEN", 1)
	if err != nil {
		t.Skipf("GEN 1 not available for %s: %v", tr.ID, err)
	}
	if ch.JSON == "" {
		t.Fatal("empty chapter json")
	}
}
