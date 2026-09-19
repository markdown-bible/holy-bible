package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DBPath                 string `yaml:"db_path"`
	DownloadURL            string `yaml:"download_url"`
	BookDir                string `yaml:"book_dir"`
	CorpusDir              string `yaml:"corpus_dir"`
	ManifestDir            string `yaml:"manifest_dir"`
	Workers                int    `yaml:"workers"`
	IncludeApocrypha       bool   `yaml:"include_apocrypha"`
	CorpusStripFootnotes   bool   `yaml:"corpus_strip_footnotes"`
	RequireRedistributable bool   `yaml:"require_redistributable"`
	SchemaExpected         string `yaml:"schema_expected"`
}

func Default() Config {
	return Config{
		DBPath:               ".cache/bible.db",
		DownloadURL:          "https://bible.helloao.org/bible.db",
		BookDir:              "book",
		CorpusDir:            "corpus",
		ManifestDir:          "manifest",
		Workers:              8,
		IncludeApocrypha:     true,
		CorpusStripFootnotes: true,
		SchemaExpected:       "schema/helloao_expected.json",
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		path = "config.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Workers < 1 {
		cfg.Workers = 1
	}
	return cfg, nil
}
