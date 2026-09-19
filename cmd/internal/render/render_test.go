package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/markdown-bible/holy-bible/cmd/internal/db"
)

func TestRenderHuman_headingAndVerse(t *testing.T) {
	chJSON := `{
		"chapter": {"number": 1},
		"content": [
			{"type": "heading", "content": ["Creation"]},
			{"type": "verse", "number": 1, "content": ["In the beginning."]},
			{"type": "verse", "number": 2, "content": [{"text": "And the earth.", "wordsOfJesus": false}]}
		]
	}`
	meta := Meta{
		TranslationID:   "BSB",
		TranslationName: "Berean Standard Bible",
		Language:        "eng",
		LicenseURL:      "https://berean.bible/",
		BookID:          "GEN",
		BookName:        "Genesis",
		Chapter:         1,
		Ref:             "GEN 1",
		DocID:           "BSB:GEN:1",
	}
	out, err := RenderHuman(chJSON, nil, meta)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Creation", "**1** In the beginning.", "# Genesis 1", "translation_id: \"BSB\""} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderCorpus_verses(t *testing.T) {
	meta := Meta{
		TranslationID:   "BSB",
		TranslationName: "Berean Standard Bible",
		Language:        "eng",
		LicenseURL:      "https://berean.bible/",
		BookID:          "GEN",
		BookName:        "Genesis",
		Chapter:         1,
		DocID:           "BSB:GEN:1",
		VerseCount:      2,
	}
	verses := []db.Verse{
		{Number: 1, Text: "In the beginning."},
		{Number: 2, Text: "The earth was formless."},
	}
	out := RenderCorpus(verses, meta)
	if !strings.Contains(out, "**1** In the beginning.") {
		t.Fatalf("unexpected corpus output: %s", out)
	}
}

func TestParseChapterJSON_footnoteRef(t *testing.T) {
	chJSON := `{
		"content": [
			{"type": "verse", "number": 3, "content": ["Hello", {"noteId": 1}]}
		],
		"footnotes": [{"noteId": 1, "text": "A note."}]
	}`
	out, err := RenderHuman(chJSON, nil, Meta{BookName: "John", Chapter: 1, BookID: "JHN"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[^1]") || !strings.Contains(out, "[^1]: A note.") {
		t.Fatalf("footnote missing: %s", out)
	}
}

func TestRenderInline_poem(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"text":"Line one","poem":1}`),
	}
	got := renderInline(raw, nil)
	if !strings.Contains(got, "> Line one") {
		t.Fatalf("poem indent missing: %q", got)
	}
}
