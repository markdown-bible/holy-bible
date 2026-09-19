package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/markdown-bible/holy-bible/cmd/internal/config"
	"github.com/markdown-bible/holy-bible/cmd/internal/db"
)

type Options struct {
	Strict bool
}

func Run(ctx context.Context, cfg config.Config, store *db.Store, opts Options) error {
	translations, err := store.ListTranslations(ctx)
	if err != nil {
		return err
	}

	var warnings []string
	for _, tr := range translations {
		if tr.LicenseURL == "" {
			return fmt.Errorf("translation %s: empty licenseUrl", tr.ID)
		}
		if !tr.LicenseNotice.Valid || tr.LicenseNotice.String == "" {
			warnings = append(warnings, fmt.Sprintf("translation %s: empty licenseNotice", tr.ID))
		}

		books, err := store.ListBooks(ctx, tr.ID, cfg.IncludeApocrypha)
		if err != nil {
			return err
		}
		for _, b := range books {
			if !cfg.IncludeApocrypha && b.IsApocryphal.Valid && b.IsApocryphal.Bool {
				return fmt.Errorf("translation %s book %s: apocryphal content with include_apocrypha=false", tr.ID, b.ID)
			}
		}

		chCount, err := store.CountChapters(ctx, tr.ID)
		if err != nil {
			return err
		}
		if chCount == 0 {
			warnings = append(warnings, fmt.Sprintf("translation %s: zero chapters", tr.ID))
		}

		lang := db.LangDir(tr.Language)
		sample := filepath.Join(cfg.BookDir, lang, tr.ID)
		if _, err := os.Stat(sample); err != nil {
			warnings = append(warnings, fmt.Sprintf("translation %s: missing book dir %s", tr.ID, sample))
		}

		licPath := filepath.Join(cfg.ManifestDir, "licenses", tr.ID+".txt")
		if _, err := os.Stat(licPath); err != nil {
			return fmt.Errorf("translation %s: missing license snapshot %s", tr.ID, licPath)
		}
	}

	manifestPath := filepath.Join(cfg.ManifestDir, "translations.json")
	if _, err := os.Stat(manifestPath); err != nil {
		return fmt.Errorf("missing %s (run build first)", manifestPath)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var manifest struct {
		Translations []struct {
			ID         string `json:"id"`
			LicenseURL string `json:"license_url"`
		} `json:"translations"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return err
	}
	for _, tr := range manifest.Translations {
		if tr.LicenseURL == "" {
			return fmt.Errorf("manifest translation %s: empty license_url", tr.ID)
		}
	}

	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
		if opts.Strict {
			return fmt.Errorf("strict validation failed: %s", w)
		}
	}
	if len(warnings) > 0 {
		fmt.Fprintf(os.Stderr, "validate: %d warning(s)\n", len(warnings))
	}
	return nil
}

func SpotCheckContent(ctx context.Context, store *db.Store, translationID, bookID string, chapter int) error {
	ch, err := store.GetChapter(ctx, translationID, bookID, chapter)
	if err != nil {
		return err
	}
	if strings.TrimSpace(ch.JSON) == "" {
		return fmt.Errorf("empty chapter json for %s %s %d", translationID, bookID, chapter)
	}
	verses, err := store.ListChapterVerses(ctx, translationID, bookID, chapter)
	if err != nil {
		return err
	}
	if len(verses) == 0 {
		return fmt.Errorf("no verses for %s %s %d", translationID, bookID, chapter)
	}
	return nil
}
