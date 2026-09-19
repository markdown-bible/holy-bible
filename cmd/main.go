package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/markdown-bible/holy-bible/cmd/internal/config"
	"github.com/markdown-bible/holy-bible/cmd/internal/db"
	"github.com/markdown-bible/holy-bible/cmd/internal/download"
	"github.com/markdown-bible/holy-bible/cmd/internal/export"
	"github.com/markdown-bible/holy-bible/cmd/internal/schema"
	"github.com/markdown-bible/holy-bible/cmd/internal/validate"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cfgPath := "config.yaml"
	if err := run(os.Args[1], os.Args[2:], cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `bible — HelloAO bible.db export tool

Usage:
  bible update [--force]
  bible sync [--force]
  bible schema check|refresh
  bible build
  bible validate [--strict]

`)
}

func run(cmd string, args []string, cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	switch cmd {
	case "update":
		fs := flag.NewFlagSet("update", flag.ExitOnError)
		force := fs.Bool("force", false, "force re-download")
		_ = fs.Parse(args)
		if err := cmdSync(cfg, *force); err != nil {
			return err
		}
		if err := cmdSchemaCheck(cfg); err != nil {
			return err
		}
		if err := cmdBuild(cfg); err != nil {
			return err
		}
		return cmdValidate(cfg, false)
	case "sync":
		fs := flag.NewFlagSet("sync", flag.ExitOnError)
		force := fs.Bool("force", false, "force re-download")
		_ = fs.Parse(args)
		return cmdSync(cfg, *force)
	case "schema":
		if len(args) == 0 {
			return fmt.Errorf("schema requires subcommand: check|refresh")
		}
		switch args[0] {
		case "check":
			return cmdSchemaCheck(cfg)
		case "refresh":
			return cmdSchemaRefresh(cfg)
		default:
			return fmt.Errorf("unknown schema subcommand: %s", args[0])
		}
	case "build":
		return cmdBuild(cfg)
	case "validate":
		fs := flag.NewFlagSet("validate", flag.ExitOnError)
		strict := fs.Bool("strict", false, "fail on warnings")
		_ = fs.Parse(args)
		return cmdValidate(cfg, *strict)
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func cmdSync(cfg config.Config, force bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()
	res, err := download.Sync(ctx, cfg.DownloadURL, cfg.DBPath, force)
	if err != nil {
		return err
	}
	if res.Skipped {
		fmt.Printf("sync: skipped (unchanged) %s sha256=%s\n", cfg.DBPath, res.SHA256)
		return nil
	}
	fmt.Printf("sync: downloaded %s (%d bytes) sha256=%s\n", cfg.DBPath, res.Bytes, res.SHA256)
	return nil
}

func cmdSchemaCheck(cfg config.Config) error {
	if _, err := os.Stat(cfg.DBPath); err != nil {
		return fmt.Errorf("database missing at %s (run: bible sync)", cfg.DBPath)
	}
	if err := schema.Check(cfg.DBPath, cfg.SchemaExpected); err != nil {
		return err
	}
	fmt.Println("schema: ok")
	return nil
}

func cmdSchemaRefresh(cfg config.Config) error {
	if _, err := os.Stat(cfg.DBPath); err != nil {
		return fmt.Errorf("database missing at %s (run: bible sync)", cfg.DBPath)
	}
	if err := os.MkdirAll("schema", 0o755); err != nil {
		return err
	}
	if err := schema.Refresh(cfg.DBPath, cfg.SchemaExpected); err != nil {
		return err
	}
	snap, _ := schema.LoadExpected(cfg.SchemaExpected)
	fmt.Printf("schema: refreshed fingerprint=%s\n", snap.Fingerprint)
	return nil
}

func cmdBuild(cfg config.Config) error {
	if _, err := os.Stat(cfg.DBPath); err != nil {
		return fmt.Errorf("database missing at %s (run: bible sync)", cfg.DBPath)
	}
	if err := schema.Check(cfg.DBPath, cfg.SchemaExpected); err != nil {
		return fmt.Errorf("%w (run: bible schema refresh after reviewing)", err)
	}

	dbSHA, err := download.DBSHA256(cfg.DBPath)
	if err != nil {
		return err
	}
	snap, err := schema.LoadExpected(cfg.SchemaExpected)
	if err != nil {
		return err
	}

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer store.Close()

	ctx := context.Background()
	res, err := export.Build(ctx, cfg, store, dbSHA, snap.Fingerprint)
	if err != nil {
		return err
	}
	fmt.Printf("build: chapters written=%d skipped=%d book_files=%d corpus_files=%d verses_jsonl=%d\n",
		res.Stats.ChaptersWritten, res.Stats.ChaptersSkipped, res.Stats.BookFiles, res.Stats.CorpusFiles, res.Stats.VerseJSONLRecords)
	return nil
}

func cmdValidate(cfg config.Config, strict bool) error {
	if _, err := os.Stat(cfg.DBPath); err != nil {
		return fmt.Errorf("database missing at %s", cfg.DBPath)
	}
	store, err := db.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer store.Close()
	ctx := context.Background()
	if err := validate.Run(ctx, cfg, store, validate.Options{Strict: strict}); err != nil {
		return err
	}
	if err := validate.SpotCheckContent(ctx, store, "BSB", "GEN", 1); err != nil {
		if strings.Contains(err.Error(), "no such") {
			fmt.Fprintf(os.Stderr, "spot check skipped: %v\n", err)
		} else {
			return err
		}
	}
	fmt.Println("validate: ok")
	return nil
}
