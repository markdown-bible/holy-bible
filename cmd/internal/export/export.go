package export

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/markdown-bible/holy-bible/cmd/internal/config"
	"github.com/markdown-bible/holy-bible/cmd/internal/db"
	"github.com/markdown-bible/holy-bible/cmd/internal/render"
)

const ExporterVersion = "0.1.0"

type BuildState struct {
	Chapters map[string]string `json:"chapters"`
}

type BuildManifest struct {
	SourceURL         string    `json:"source_url"`
	BibleDBSHA256     string    `json:"bible_db_sha256"`
	SchemaFingerprint string    `json:"schema_fingerprint,omitempty"`
	ExporterVersion   string    `json:"exporter_version"`
	BuiltAt           time.Time `json:"built_at"`
}

type TranslationRecord struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	EnglishName     string `json:"english_name"`
	ShortName       string `json:"short_name,omitempty"`
	Language        string `json:"language"`
	Website         string `json:"website,omitempty"`
	LicenseURL      string `json:"license_url"`
	LicenseNotes    string `json:"license_notes,omitempty"`
	LicenseNotice   string `json:"license_notice,omitempty"`
	LicenseJSON     string `json:"license_json,omitempty"`
	ContentSHA256   string `json:"content_sha256,omitempty"`
	BookDir         string `json:"book_dir"`
	CorpusDir       string `json:"corpus_dir"`
	ChapterCount    int    `json:"chapter_count"`
	VerseCount      int    `json:"verse_count"`
	Redistributable *bool  `json:"redistributable,omitempty"`
}

type Stats struct {
	Translations      int   `json:"translations"`
	ChaptersWritten   int64 `json:"chapters_written"`
	ChaptersSkipped   int64 `json:"chapters_skipped"`
	BookFiles         int64 `json:"book_files"`
	CorpusFiles       int64 `json:"corpus_files"`
	VerseJSONLRecords int64 `json:"verse_jsonl_records"`
}

type Result struct {
	Manifest BuildManifest
	Stats    Stats
}

type job struct {
	translation db.Translation
	book        db.Book
	chapterNum  int
}

func Build(ctx context.Context, cfg config.Config, store *db.Store, dbSHA256, schemaFP string) (Result, error) {
	var res Result
	res.Manifest = BuildManifest{
		SourceURL:         cfg.DownloadURL,
		BibleDBSHA256:     dbSHA256,
		SchemaFingerprint: schemaFP,
		ExporterVersion:   ExporterVersion,
		BuiltAt:           time.Now().UTC(),
	}

	statePath := filepath.Join(cfg.ManifestDir, "build.state.json")
	prevState := loadState(statePath)
	newState := BuildState{Chapters: map[string]string{}}
	var stateMu sync.Mutex

	if err := os.MkdirAll(cfg.ManifestDir, 0o755); err != nil {
		return res, err
	}
	if err := os.MkdirAll(filepath.Join(cfg.ManifestDir, "licenses"), 0o755); err != nil {
		return res, err
	}

	translations, err := store.ListTranslations(ctx)
	if err != nil {
		return res, err
	}
	res.Stats.Translations = len(translations)

	jobs := make(chan job, cfg.Workers*4)
	var wg sync.WaitGroup
	var chaptersWritten, chaptersSkipped, bookFiles, corpusFiles atomic.Int64

	worker := func() {
		defer wg.Done()
		for j := range jobs {
			if ctx.Err() != nil {
				return
			}
			key := chapterKey(j.translation.ID, j.book.ID, j.chapterNum)
			ch, err := store.GetChapter(ctx, j.translation.ID, j.book.ID, j.chapterNum)
			if err != nil {
				continue
			}
			chHash := ""
			if ch.SHA256.Valid {
				chHash = ch.SHA256.String
			}
			stateMu.Lock()
			newState.Chapters[key] = chHash
			stateMu.Unlock()
			if prevHash, ok := prevState.Chapters[key]; ok && prevHash == chHash && chHash != "" {
				chaptersSkipped.Add(1)
				continue
			}

			verses, err := store.ListChapterVerses(ctx, j.translation.ID, j.book.ID, j.chapterNum)
			if err != nil {
				continue
			}
			footnotes, _ := store.ListChapterFootnotes(ctx, j.translation.ID, j.book.ID, j.chapterNum)

			lang := db.LangDir(j.translation.Language)
			bookPath := filepath.Join(cfg.BookDir, lang, j.translation.ID, j.book.ID, db.ChapterFileName(j.chapterNum))
			corpusPath := filepath.Join(cfg.CorpusDir, lang, j.translation.ID, j.book.ID, db.ChapterFileName(j.chapterNum))

			notice := ""
			if j.translation.LicenseNotice.Valid {
				notice = j.translation.LicenseNotice.String
			}
			meta := render.Meta{
				TranslationID:   j.translation.ID,
				TranslationName: j.translation.Name,
				Language:        j.translation.Language,
				LicenseURL:      j.translation.LicenseURL,
				LicenseNotice:   notice,
				BookID:          j.book.ID,
				BookName:        j.book.Name,
				Chapter:         j.chapterNum,
				Ref:             fmt.Sprintf("%s %d", j.book.ID, j.chapterNum),
				SourceDBSHA256:  dbSHA256,
				ChapterSHA256:   chHash,
				VerseCount:      len(verses),
				DocID:           fmt.Sprintf("%s:%s:%d", j.translation.ID, j.book.ID, j.chapterNum),
			}

			human, err := render.RenderHuman(ch.JSON, footnotes, meta)
			if err != nil {
				human = render.RenderCorpus(verses, meta)
			}
			corpus := render.RenderCorpus(verses, meta)

			if err := writeFile(bookPath, human); err != nil {
				continue
			}
			if err := writeFile(corpusPath, corpus); err != nil {
				continue
			}
			bookFiles.Add(1)
			corpusFiles.Add(1)
			chaptersWritten.Add(1)
		}
	}

	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go worker()
	}

	var records []TranslationRecord
	var verseRecords int64

	for _, tr := range translations {
		if tr.LicenseURL == "" {
			return res, fmt.Errorf("translation %s missing licenseUrl", tr.ID)
		}
		if cfg.RequireRedistributable {
			rd, err := store.Redistributable(ctx, tr.ID)
			if err != nil {
				return res, err
			}
			if rd.Valid && !rd.Bool {
				return res, fmt.Errorf("translation %s not redistributable", tr.ID)
			}
		}

		books, err := store.ListBooks(ctx, tr.ID, cfg.IncludeApocrypha)
		if err != nil {
			return res, err
		}
		chCount, err := store.CountChapters(ctx, tr.ID)
		if err != nil {
			return res, err
		}
		vCount, err := store.CountVerses(ctx, tr.ID)
		if err != nil {
			return res, err
		}

		lang := db.LangDir(tr.Language)
		rec := TranslationRecord{
			ID:            tr.ID,
			Name:          tr.Name,
			EnglishName:   tr.EnglishName,
			ShortName:     tr.ShortName,
			Language:      tr.Language,
			Website:       tr.Website,
			LicenseURL:    tr.LicenseURL,
			BookDir:       filepath.ToSlash(filepath.Join(cfg.BookDir, lang, tr.ID)),
			CorpusDir:     filepath.ToSlash(filepath.Join(cfg.CorpusDir, lang, tr.ID)),
			ChapterCount:  chCount,
			VerseCount:    vCount,
		}
		if tr.LicenseNotes.Valid {
			rec.LicenseNotes = tr.LicenseNotes.String
		}
		if tr.LicenseNotice.Valid {
			rec.LicenseNotice = tr.LicenseNotice.String
		}
		if tr.SHA256.Valid {
			rec.ContentSHA256 = tr.SHA256.String
		}
		rd, _ := store.Redistributable(ctx, tr.ID)
		if rd.Valid {
			v := rd.Bool
			rec.Redistributable = &v
		}
		records = append(records, rec)

		if err := writeLicenseSnapshot(cfg, tr, dbSHA256); err != nil {
			return res, err
		}

		jsonlPath := filepath.Join(cfg.CorpusDir, lang, tr.ID, "verses.jsonl")
		if err := os.MkdirAll(filepath.Dir(jsonlPath), 0o755); err != nil {
			return res, err
		}
		jf, err := os.Create(jsonlPath)
		if err != nil {
			return res, err
		}
		enc := json.NewEncoder(jf)

		for _, book := range books {
			for ch := 1; ch <= book.NumberOfChapters; ch++ {
				jobs <- job{translation: tr, book: book, chapterNum: ch}
			}
			for ch := 1; ch <= book.NumberOfChapters; ch++ {
				verses, err := store.ListChapterVerses(ctx, tr.ID, book.ID, ch)
				if err != nil {
					continue
				}
				for _, v := range verses {
					row := map[string]any{
						"id":              fmt.Sprintf("%s:%s:%d:%d", tr.ID, book.ID, ch, v.Number),
						"translation_id":  tr.ID,
						"language":        tr.Language,
						"book_id":         book.ID,
						"chapter":         ch,
						"verse":           v.Number,
						"text":            strings.TrimSpace(v.Text),
						"license_url":     tr.LicenseURL,
					}
					if err := enc.Encode(row); err != nil {
						_ = jf.Close()
						return res, err
					}
					verseRecords++
				}
			}
		}
		_ = jf.Close()
	}

	close(jobs)
	wg.Wait()

	res.Stats.ChaptersWritten = chaptersWritten.Load()
	res.Stats.ChaptersSkipped = chaptersSkipped.Load()
	res.Stats.BookFiles = bookFiles.Load()
	res.Stats.CorpusFiles = corpusFiles.Load()
	res.Stats.VerseJSONLRecords = verseRecords

	if err := saveState(statePath, newState); err != nil {
		return res, err
	}
	if err := writeJSON(filepath.Join(cfg.ManifestDir, "build.json"), res.Manifest); err != nil {
		return res, err
	}
	if err := writeJSON(filepath.Join(cfg.ManifestDir, "stats.json"), res.Stats); err != nil {
		return res, err
	}
	if err := writeJSON(filepath.Join(cfg.ManifestDir, "translations.json"), map[string]any{"translations": records}); err != nil {
		return res, err
	}

	return res, nil
}

func chapterKey(translationID, bookID string, chapter int) string {
	return fmt.Sprintf("%s:%s:%d", translationID, bookID, chapter)
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".part"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func loadState(path string) BuildState {
	st := BuildState{Chapters: map[string]string{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return st
	}
	_ = json.Unmarshal(data, &st)
	if st.Chapters == nil {
		st.Chapters = map[string]string{}
	}
	return st
}

func saveState(path string, st BuildState) error {
	return writeJSON(path, st)
}

func writeLicenseSnapshot(cfg config.Config, tr db.Translation, dbSHA256 string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Copied from HelloAO bible.db; db sha256 %s\n\n", dbSHA256)
	fmt.Fprintf(&b, "Translation: %s (%s)\n", tr.Name, tr.ID)
	fmt.Fprintf(&b, "License URL: %s\n\n", tr.LicenseURL)
	if tr.LicenseNotice.Valid && tr.LicenseNotice.String != "" {
		b.WriteString(tr.LicenseNotice.String)
		b.WriteString("\n")
	} else if tr.LicenseNotes.Valid {
		b.WriteString(tr.LicenseNotes.String)
		b.WriteString("\n")
	}
	path := filepath.Join(cfg.ManifestDir, "licenses", tr.ID+".txt")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
