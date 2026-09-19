# holy-bible

Markdown corpus of Bible translations exported from the [Free Use Bible API](https://bible.helloao.org/) SQLite database (`bible.db`).

## Layout

| Path | Purpose |
|------|---------|
| `book/` | Human-readable markdown (headings, poetry, footnotes) |
| `corpus/` | RAG-oriented markdown + `verses.jsonl` per translation |
| `manifest/` | Build metadata, translation catalog, license snapshots |
| `.cache/` | Local `bible.db` (not committed) |

See [LEGAL.md](LEGAL.md) for licensing (Apache 2.0 applies to **tooling only**; scripture is per-translation).

## Requirements

- Go 1.22+
- ~15 GB free disk (11 GB database + export headroom)
- Network access for initial `bible sync`

## Build the CLI

```bash
go build -o bible ./cmd
```

## Full update pipeline

```bash
./bible update          # sync → schema check → build → validate
./bible sync            # download bible.db when changed
./bible schema refresh  # after first sync or HelloAO schema change
./bible schema check
./bible build
./bible validate        # add --strict to fail on warnings
```

Configuration: [`config.yaml`](config.yaml).

## Citation

Use translation name, reference (e.g. `GEN 1:1`), and `license_url` from file front matter or `manifest/translations.json`.

## Tests

```bash
go test ./... -race
```

Integration tests (require `.cache/bible.db`):

```bash
go test ./... -race -tags=integration
```

## Upstream

- Database: https://bible.helloao.org/bible.db  
- Schema reference: [HelloAOLab schema.prisma](https://github.com/HelloAOLab/bible-api/blob/main/packages/helloao-cli/schema.prisma)
