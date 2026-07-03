package ingest

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	doczcfg "github.com/donaldgifford/docz/pkg/doczcore/config"
)

// loadConfig parses fetched .docz.yaml bytes. doczcfg.Load is filesystem-based
// (it needs a repo root on disk), so the bytes are written to a private temp
// dir and loaded from there; the dir is removed before returning. Doc blobs
// never touch disk — they are parsed byte-wise via doczdoc.ParseFrontmatter.
//
// doczcfg.Load merges $HOME/.docz.yaml when present. In the container there is
// no such file, so nothing is merged; tests neutralize HOME with
// t.Setenv("HOME", t.TempDir()) to stay hermetic against a developer's config.
func loadConfig(configYAML []byte) (doczcfg.Config, error) {
	dir, err := os.MkdirTemp("", "docz-ingest-*")
	if err != nil {
		return doczcfg.Config{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		if rerr := os.RemoveAll(dir); rerr != nil {
			slog.Warn("remove ingest temp dir", "dir", dir, "err", rerr)
		}
	}()

	if err := os.WriteFile(filepath.Join(dir, ".docz.yaml"), configYAML, 0o600); err != nil {
		return doczcfg.Config{}, fmt.Errorf("write .docz.yaml: %w", err)
	}

	cfg, err := doczcfg.Load("", dir)
	if err != nil {
		return doczcfg.Config{}, fmt.Errorf("load .docz.yaml: %w", err)
	}
	return cfg, nil
}
