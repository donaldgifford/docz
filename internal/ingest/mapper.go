package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/donaldgifford/docz-api/internal/store"
	doczcfg "github.com/donaldgifford/docz/pkg/doczcore/config"
	doczdoc "github.com/donaldgifford/docz/pkg/doczcore/document"
)

// createdLayout is the docz frontmatter date format ("2026-01-15").
const createdLayout = "2006-01-02"

// mapDocType converts one doczcfg.TypeConfig into a store.DocTypeInput. name is
// the canonical type name (the key in cfg.Types). Statuses and Aliases become
// JSONB payloads.
func mapDocType(name string, tc *doczcfg.TypeConfig) (store.DocTypeInput, error) {
	statuses, err := json.Marshal(tc.Statuses)
	if err != nil {
		return store.DocTypeInput{}, fmt.Errorf("marshal statuses for type %q: %w", name, err)
	}
	aliases, err := json.Marshal(tc.Aliases)
	if err != nil {
		return store.DocTypeInput{}, fmt.Errorf("marshal aliases for type %q: %w", name, err)
	}
	return store.DocTypeInput{
		Name:        name,
		Dir:         tc.Dir,
		IDPrefix:    tc.IDPrefix,
		PluralLabel: tc.PluralLabel,
		Statuses:    statuses,
		Aliases:     aliases,
	}, nil
}

// mapDocument converts a fetched blob and its parsed frontmatter into a
// store.DocumentInput. typeName is the canonical type. The content hash is the
// hex sha256 of the raw blob bytes; Created is parsed from the frontmatter date
// (zero time when empty, which the store maps to SQL NULL).
func mapDocument(typeName string, blob *BlobEntry, fm *doczdoc.Frontmatter) (store.DocumentInput, error) {
	sum := sha256.Sum256(blob.Content)

	var created time.Time
	if fm.Created != "" {
		t, err := time.Parse(createdLayout, fm.Created)
		if err != nil {
			return store.DocumentInput{}, fmt.Errorf("parse created %q for %s: %w", fm.Created, fm.ID, err)
		}
		created = t
	}

	return store.DocumentInput{
		Type:        typeName,
		DocID:       fm.ID,
		Title:       fm.Title,
		Status:      string(fm.Status),
		Author:      fm.Author,
		Created:     created,
		Path:        blob.Path,
		GitSHA:      blob.GitSHA,
		ContentHash: hex.EncodeToString(sum[:]),
		RawMD:       string(blob.Content),
	}, nil
}
