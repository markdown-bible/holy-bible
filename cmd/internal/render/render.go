package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/markdown-bible/holy-bible/cmd/internal/db"
)

type chapterPayload struct {
	Translation json.RawMessage   `json:"translation"`
	Book        json.RawMessage   `json:"book"`
	Chapter     chapterMeta       `json:"chapter"`
	Footnotes   []chapterFootnote `json:"footnotes"`
	Content     []chapterContent  `json:"content"`
}

type chapterMeta struct {
	Number int    `json:"number"`
	Ref    string `json:"reference,omitempty"`
}

type chapterFootnote struct {
	NoteID int    `json:"noteId"`
	Text   string `json:"text"`
	Caller string `json:"caller,omitempty"`
}

type chapterContent struct {
	Type string `json:"type"`

	Content []json.RawMessage `json:"content,omitempty"`
	Number  int               `json:"number,omitempty"`

	Heading string `json:"heading,omitempty"`
}

type formattedText struct {
	Text          string `json:"text"`
	Poem          *int   `json:"poem,omitempty"`
	WordsOfJesus  *bool  `json:"wordsOfJesus,omitempty"`
}

type footnoteRef struct {
	NoteID int `json:"noteId"`
}

type inlineHeading struct {
	Heading string `json:"heading"`
}

type inlineLineBreak struct {
	LineBreak bool `json:"lineBreak"`
}

type Meta struct {
	TranslationID   string
	TranslationName string
	Language        string
	LicenseURL      string
	LicenseNotice   string
	BookID          string
	BookName        string
	Chapter         int
	Ref             string
	SourceDBSHA256  string
	ChapterSHA256   string
	VerseCount      int
	DocID           string
}

func ParseChapterJSON(raw string) (chapterPayload, error) {
	var p chapterPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return p, err
	}
	if len(p.Content) == 0 {
		var alt struct {
			Content []chapterContent `json:"content"`
		}
		if err := json.Unmarshal([]byte(raw), &alt); err == nil && len(alt.Content) > 0 {
			p.Content = alt.Content
		}
	}
	if len(p.Content) == 0 {
		var only []chapterContent
		if err := json.Unmarshal([]byte(raw), &only); err == nil && len(only) > 0 {
			p.Content = only
		}
	}
	return p, nil
}

func RenderHuman(chJSON string, footnotes []db.Footnote, meta Meta) (string, error) {
	p, err := ParseChapterJSON(chJSON)
	if err != nil {
		return "", err
	}

	fnMap := map[int]string{}
	for _, f := range footnotes {
		fnMap[f.ID] = f.Text
	}
	for _, f := range p.Footnotes {
		fnMap[f.NoteID] = f.Text
	}

	var b strings.Builder
	writeFrontMatter(&b, meta, "human")
	b.WriteString("\n")

	title := meta.BookName
	if title == "" {
		title = meta.BookID
	}
	fmt.Fprintf(&b, "# %s %d\n\n", title, meta.Chapter)

	for _, piece := range p.Content {
		switch piece.Type {
		case "heading":
			text := concatHeading(piece.Content)
			if text != "" {
				fmt.Fprintf(&b, "## %s\n\n", text)
			}
		case "hebrew_subtitle":
			text := renderInline(piece.Content, fnMap)
			if text != "" {
				fmt.Fprintf(&b, "*%s*\n\n", strings.TrimSpace(text))
			}
		case "line_break":
			b.WriteByte('\n')
		case "verse":
			line := renderVerseHuman(piece, fnMap)
			fmt.Fprintf(&b, "**%d** %s\n\n", piece.Number, line)
		}
	}

	if len(fnMap) > 0 {
		b.WriteString("## Footnotes\n\n")
		ids := sortedKeys(fnMap)
		for _, id := range ids {
			text := fnMap[id]
			if text == "" {
				continue
			}
			fmt.Fprintf(&b, "[^%d]: %s\n", id, text)
		}
	}

	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

func RenderCorpus(verses []db.Verse, meta Meta) string {
	var b strings.Builder
	writeFrontMatter(&b, meta, "corpus")
	b.WriteString("\n")
	title := meta.BookName
	if title == "" {
		title = meta.BookID
	}
	fmt.Fprintf(&b, "# %s %d\n\n", title, meta.Chapter)
	for _, v := range verses {
		text := strings.TrimSpace(v.Text)
		if text == "" {
			continue
		}
		fmt.Fprintf(&b, "**%d** %s\n\n", v.Number, text)
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func writeFrontMatter(b *strings.Builder, meta Meta, kind string) {
	b.WriteString("---\n")
	writeYAML(b, "format", kind)
	writeYAML(b, "translation_id", meta.TranslationID)
	writeYAML(b, "translation_name", meta.TranslationName)
	writeYAML(b, "language", meta.Language)
	writeYAML(b, "license_url", meta.LicenseURL)
	if meta.LicenseNotice != "" {
		writeYAML(b, "license_notice", meta.LicenseNotice)
	}
	writeYAML(b, "book_id", meta.BookID)
	writeYAML(b, "book_name", meta.BookName)
	writeYAML(b, "chapter", fmt.Sprintf("%d", meta.Chapter))
	if meta.Ref != "" {
		writeYAML(b, "ref", meta.Ref)
	}
	if meta.DocID != "" {
		writeYAML(b, "doc_id", meta.DocID)
	}
	if meta.VerseCount > 0 {
		writeYAML(b, "verse_count", fmt.Sprintf("%d", meta.VerseCount))
	}
	if meta.SourceDBSHA256 != "" {
		writeYAML(b, "source_db_sha256", meta.SourceDBSHA256)
	}
	if meta.ChapterSHA256 != "" {
		writeYAML(b, "chapter_sha256", meta.ChapterSHA256)
	}
	b.WriteString("---\n")
}

func writeYAML(b *strings.Builder, key, value string) {
	value = strings.ReplaceAll(value, "\n", " ")
	fmt.Fprintf(b, "%s: %q\n", key, value)
}

func renderVerseHuman(piece chapterContent, fnMap map[int]string) string {
	return renderInline(piece.Content, fnMap)
}

func renderInline(parts []json.RawMessage, fnMap map[int]string) string {
	var b strings.Builder
	for _, raw := range parts {
		if len(raw) == 0 {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			b.WriteString(s)
			continue
		}
		var ft formattedText
		if err := json.Unmarshal(raw, &ft); err == nil && ft.Text != "" {
			text := ft.Text
			if ft.WordsOfJesus != nil && *ft.WordsOfJesus {
				text = "*" + text + "*"
			}
			if ft.Poem != nil && *ft.Poem > 0 {
				prefix := strings.Repeat("> ", *ft.Poem)
				text = prefix + text
			}
			b.WriteString(text)
			continue
		}
		var ih inlineHeading
		if err := json.Unmarshal(raw, &ih); err == nil && ih.Heading != "" {
			b.WriteString(ih.Heading)
			continue
		}
		var lb inlineLineBreak
		if err := json.Unmarshal(raw, &lb); err == nil && lb.LineBreak {
			b.WriteString("\n")
			continue
		}
		var fn footnoteRef
		if err := json.Unmarshal(raw, &fn); err == nil && fn.NoteID > 0 {
			fmt.Fprintf(&b, "[^%d]", fn.NoteID)
			continue
		}
	}
	return strings.TrimSpace(b.String())
}

func concatHeading(parts []json.RawMessage) string {
	var ss []string
	for _, raw := range parts {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			ss = append(ss, s)
		}
	}
	return strings.TrimSpace(strings.Join(ss, " "))
}

func sortedKeys(m map[int]string) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
