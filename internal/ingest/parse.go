package ingest

import (
	"fmt"

	doczcfg "github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// loadConfig parses fetched .docz.yaml bytes with doczcfg.ParseBytes: the same
// decode and normalisation doczcfg.Load applies, with no file written and no
// $HOME/.docz.yaml merged (IMPL-0019 Phase 3). Doc blobs never touch disk
// either — they are parsed byte-wise via doczdoc.ParseFrontmatter.
func loadConfig(configYAML []byte) (doczcfg.Config, error) {
	cfg, err := doczcfg.ParseBytes(configYAML)
	if err != nil {
		return doczcfg.Config{}, fmt.Errorf("load .docz.yaml: %w", err)
	}
	return cfg, nil
}
