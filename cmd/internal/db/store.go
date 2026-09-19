package db

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

type Translation struct {
	ID            string
	Name          string
	EnglishName   string
	ShortName     string
	Language      string
	Website       string
	LicenseURL    string
	LicenseNotes  sql.NullString
	LicenseNotice sql.NullString
	SHA256        sql.NullString
	TextDirection string
}

type Book struct {
	TranslationID    string
	ID               string
	Name             string
	CommonName       string
	Order            int
	NumberOfChapters int
	IsApocryphal     sql.NullBool
}

type Chapter struct {
	TranslationID string
	BookID        string
	Number        int
	JSON          string
	SHA256        sql.NullString
}

type Verse struct {
	TranslationID string
	BookID        string
	ChapterNumber int
	Number        int
	Text          string
	ContentJSON   string
}

type Footnote struct {
	TranslationID string
	BookID        string
	ChapterNumber int
	ID            int
	Text          string
	VerseNumber   sql.NullInt64
}

func (s *Store) ListTranslations(ctx context.Context) ([]Translation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, englishName, shortName, language, website,
		       licenseUrl, licenseNotes, licenseNotice, sha256, textDirection
		FROM Translation ORDER BY language, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Translation
	for rows.Next() {
		var t Translation
		if err := rows.Scan(
			&t.ID, &t.Name, &t.EnglishName, &t.ShortName, &t.Language, &t.Website,
			&t.LicenseURL, &t.LicenseNotes, &t.LicenseNotice, &t.SHA256, &t.TextDirection,
		); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) Redistributable(ctx context.Context, translationID string) (sql.NullBool, error) {
	var v sql.NullBool
	err := s.db.QueryRowContext(ctx, `
		SELECT redistributable FROM EBibleSource WHERE translationId = ? LIMIT 1`, translationID).Scan(&v)
	if err == sql.ErrNoRows {
		return sql.NullBool{}, nil
	}
	return v, err
}

func (s *Store) ListBooks(ctx context.Context, translationID string, includeApocrypha bool) ([]Book, error) {
	q := `
		SELECT translationId, id, name, commonName, "order", numberOfChapters, isApocryphal
		FROM Book WHERE translationId = ?`
	if !includeApocrypha {
		q += ` AND (isApocryphal IS NULL OR isApocryphal = 0)`
	}
	q += ` ORDER BY "order"`
	rows, err := s.db.QueryContext(ctx, q, translationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Book
	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.TranslationID, &b.ID, &b.Name, &b.CommonName, &b.Order, &b.NumberOfChapters, &b.IsApocryphal); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) GetChapter(ctx context.Context, translationID, bookID string, number int) (Chapter, error) {
	var c Chapter
	err := s.db.QueryRowContext(ctx, `
		SELECT translationId, bookId, number, json, sha256
		FROM Chapter WHERE translationId = ? AND bookId = ? AND number = ?`,
		translationID, bookID, number,
	).Scan(&c.TranslationID, &c.BookID, &c.Number, &c.JSON, &c.SHA256)
	return c, err
}

func (s *Store) ListChapterVerses(ctx context.Context, translationID, bookID string, chapter int) ([]Verse, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT translationId, bookId, chapterNumber, number, text, contentJson
		FROM ChapterVerse
		WHERE translationId = ? AND bookId = ? AND chapterNumber = ?
		ORDER BY number`, translationID, bookID, chapter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Verse
	for rows.Next() {
		var v Verse
		if err := rows.Scan(&v.TranslationID, &v.BookID, &v.ChapterNumber, &v.Number, &v.Text, &v.ContentJSON); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) ListChapterFootnotes(ctx context.Context, translationID, bookID string, chapter int) ([]Footnote, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT translationId, bookId, chapterNumber, id, text, verseNumber
		FROM ChapterFootnote
		WHERE translationId = ? AND bookId = ? AND chapterNumber = ?
		ORDER BY id`, translationID, bookID, chapter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Footnote
	for rows.Next() {
		var f Footnote
		if err := rows.Scan(&f.TranslationID, &f.BookID, &f.ChapterNumber, &f.ID, &f.Text, &f.VerseNumber); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) CountChapters(ctx context.Context, translationID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM Chapter WHERE translationId = ?`, translationID).Scan(&n)
	return n, err
}

func (s *Store) CountVerses(ctx context.Context, translationID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ChapterVerse WHERE translationId = ?`, translationID).Scan(&n)
	return n, err
}

func TranslationRoot(base, translationID string) string {
	return filepath.Join(base, translationID)
}

func ChapterFileName(number int) string {
	return fmt.Sprintf("%02d.md", number)
}

func LangDir(language string) string {
	if language == "" {
		return "und"
	}
	return language
}
