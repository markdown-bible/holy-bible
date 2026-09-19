package clean

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/markdown-bible/holy-bible/cmd/internal/config"
	"github.com/markdown-bible/holy-bible/cmd/internal/db"
)

type Options struct {
	Languages      []string
	LegacyLangDirs bool
	UntrackedOnly  bool
}

// Run removes export trees. With UntrackedOnly, only deletes paths not in git (via git clean).
// With LegacyLangDirs, removes book/{lang}/ and corpus/{lang}/ wrapper directories.
func Run(ctx context.Context, cfg config.Config, store *db.Store, opts Options) error {
	if opts.UntrackedOnly {
		return fmt.Errorf("use: git clean -fd book corpus manifest (untracked export from full builds)")
	}

	langs := opts.Languages
	if len(langs) == 0 {
		langs = cfg.Languages
	}

	translations, err := store.ListTranslations(ctx)
	if err != nil {
		return err
	}

	var targets []db.Translation
	for _, tr := range translations {
		if len(langs) == 0 {
			targets = append(targets, tr)
			continue
		}
		for _, l := range langs {
			if tr.Language == l || tr.ID == l {
				targets = append(targets, tr)
				break
			}
		}
	}

	for _, tr := range targets {
		for _, root := range []string{cfg.BookDir, cfg.CorpusDir} {
			p := db.TranslationRoot(root, tr.ID)
			if err := os.RemoveAll(p); err != nil {
				return fmt.Errorf("remove %s: %w", p, err)
			}
		}
		lic := filepath.Join(cfg.ManifestDir, "licenses", tr.ID+".txt")
		_ = os.Remove(lic)
	}

	if opts.LegacyLangDirs || len(langs) > 0 {
		langSet := map[string]struct{}{}
		if len(langs) > 0 {
			for _, l := range langs {
				langSet[l] = struct{}{}
			}
		} else {
			for _, tr := range targets {
				langSet[db.LangDir(tr.Language)] = struct{}{}
			}
		}
		for lang := range langSet {
			for _, root := range []string{cfg.BookDir, cfg.CorpusDir} {
				p := filepath.Join(root, lang)
				if err := os.RemoveAll(p); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
