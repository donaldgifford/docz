package httpapi

import (
	"encoding/json"
	"testing"

	"github.com/donaldgifford/docz-api/internal/store"
)

func TestResolveType(t *testing.T) {
	types := []store.DocType{
		{Name: "frameworks", IDPrefix: "FW", Aliases: json.RawMessage(`["framework"]`)},
		{Name: "rfc", IDPrefix: "RFC", Aliases: json.RawMessage(`[]`)},
	}
	tests := []struct {
		input    string
		want     string
		wantOK   bool
		wantWhat string
	}{
		{"frameworks", "frameworks", true, "canonical name"},
		{"FW", "frameworks", true, "id_prefix"},
		{"fw", "frameworks", true, "id_prefix case-insensitive"},
		{"framework", "frameworks", true, "alias"},
		{"rfc", "rfc", true, "second type name"},
		{"bogus", "", false, "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.wantWhat, func(t *testing.T) {
			got, ok := resolveType(types, tt.input)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("resolveType(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
