# holy-bible

Markdown corpus of Bible translations exported from the [Free Use Bible API](https://bible.helloao.org/) SQLite database (`bible.db`).

## Layout

Paths are **flat by translation id** (no `{lang}/` prefix):

```text
book/{translationId}/{bookId}/{chapter}.md
corpus/{translationId}/{bookId}/{chapter}.md
corpus/{translationId}/verses.jsonl
manifest/translations.json
manifest/licenses/{translationId}.txt
```

Example: `book/BSB/GEN/01.md`, `book/asm_irv/MAT/05.md`.

Legacy trees `book/eng/BSB/...` can be removed after rebuilding with `./bible clean -lang eng -legacy-lang-dirs`.

See [LEGAL.md](LEGAL.md) for licensing (Apache 2.0 applies to **tooling only**; scripture is per-translation).

## Requirements

- Go 1.22+
- ~15 GB free disk (11 GB database + export headroom)
- Network access for initial `bible sync`

## Build the CLI

```bash
go build -o bible ./cmd
```

## Workflow (one language at a time)

```bash
./bible sync
./bible schema check

# Remove a failed or old layout (optional)
git clean -fd book corpus    # drop untracked bulk export only
./bible clean -lang eng      # remove eng translations + book/eng/ legacy dirs

# Export one language
./bible build -lang eng
./bible validate -lang eng

# Commit selectively (avoid git add .)
git add book/BSB corpus/BSB manifest/licenses/BSB.txt
git add manifest/translations.json manifest/build.json manifest/stats.json
```

Build several languages by repeating `-lang` or set `languages: [eng, asm]` in `config.yaml`.

Full pipeline (all translations — large):

```bash
./bible update
```

## Commands

| Command | Description |
|---------|-------------|
| `./bible build -lang eng` | Export only matching language codes |
| `./bible clean -lang eng` | Delete export dirs for that language (+ legacy `book/eng/`) |
| `./bible validate -lang eng` | Validate only those translations |

Configuration: [`config.yaml`](config.yaml).

## Citation

Use translation name, reference (e.g. `GEN 1:1`), and `license_url` from front matter or `manifest/translations.json`.

## Tests

```bash
go test ./... -race
```

## Upstream

- Database: https://bible.helloao.org/bible.db  
- Schema reference: [HelloAOLab schema.prisma](https://github.com/HelloAOLab/bible-api/blob/main/packages/helloao-cli/schema.prisma)
