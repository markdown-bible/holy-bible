# Legal information

This document is not legal advice. It explains how licensing works in this
repository for humans and automated systems (including AI/RAG pipelines).

## Composite work

| Layer | Path | License |
|-------|------|---------|
| Export tooling | `cmd/`, `config.yaml`, `schema/` | Apache 2.0 ([LICENSE](LICENSE), [NOTICE](NOTICE)) |
| Scripture text | `book/`, `corpus/` | **Per translation** (see below) |
| Catalog | `manifest/translations.json`, `manifest/licenses/` | Metadata only; points to translation terms |

Apache 2.0 does **not** apply to Bible text in `book/` or `corpus/`.

## Per-translation terms

Every exported translation includes:

- `license_url` in YAML front matter and in `corpus/.../verses.jsonl`
- A snapshot file at `manifest/licenses/{translationId}.txt`

Before redistributing, training models, or shipping a commercial product using
a translation, read that translation's `license_url` and notice text.

## Citations

When quoting a verse, include:

1. Translation name (e.g. Berean Standard Bible)
2. Reference (e.g. Genesis 1:1)
3. `license_url` from front matter or `manifest/translations.json`

## Human vs corpus exports

- **`book/`** — formatted markdown (headings, poetry, footnotes) for reading.
- **`corpus/`** — plain verse text optimized for retrieval; footnotes may be
  omitted in chapter bodies but underlying verse text comes from the database
  `ChapterVerse.text` field without editorial changes.

We do not alter verse wording in the export pipeline. Format-only conversion.

## AI use

- **RAG / retrieval**: pass through `license_url` (and attribution when required)
  with each chunk you store or display.
- **Model training**: terms vary by translation (public domain, CC, custom).
  This monorepo contains **many** translations with **mixed** terms. Do not
  assume one rule covers the entire repository.

## Upstream

Content is exported from [HelloAO `bible.db`](https://bible.helloao.org/bible.db).
See HelloAO's [licensing essay](https://bible.helloao.org/docs/guide/a-biblical-model-for-licensing-the-bible.html).

If you modify scripture text, publish it under a **different translation name**
(HelloAO and Berean project norms).

## Removed translations

If a translation disappears from a future upstream database, this project may
retain the last export and mark it `deprecated` in `manifest/translations.json`
for stable citations.
