package githubapp

import (
	"encoding/base64"
	"fmt"
	"io"
	"maps"
	"net/http"
	"path"
	"strings"
	"testing"

	"github.com/google/go-github/v88/github"
)

// stubTransport serves canned GitHub API responses keyed off the request path,
// so Fetch can be exercised without a network or a token exchange.
type stubTransport struct {
	tree  string
	blobs map[string]string // blob sha -> base64-encoded content
}

func (s stubTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	p := r.URL.Path
	switch {
	case strings.Contains(p, "/git/ref"):
		return jsonResponse(`{"ref":"refs/heads/main","object":{"sha":"headsha","type":"commit"}}`), nil
	case strings.Contains(p, "/git/trees/"):
		return jsonResponse(s.tree), nil
	case strings.Contains(p, "/git/blobs/"):
		sha := path.Base(p)
		content, ok := s.blobs[sha]
		if !ok {
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(`{}`)), Header: jsonHeader()}, nil
		}
		return jsonResponse(fmt.Sprintf(`{"sha":%q,"encoding":"base64","content":%q}`, sha, content)), nil
	default:
		return jsonResponse(`{"name":"platform","default_branch":"main"}`), nil
	}
}

func jsonHeader() http.Header {
	h := http.Header{}
	h.Set("Content-Type", "application/json")
	return h
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     jsonHeader(),
	}
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func TestFetchClassifiesAndDecodes(t *testing.T) {
	const (
		cfgYAML  = "docs_dir: docs\ntypes:\n  frameworks:\n    enabled: true\n"
		changelo = "# Changelog\n\n- init\n"
		docBody  = "---\nid: FW-0001\ntitle: Intro\n---\n\n# Intro\n"
	)
	tree := `{"sha":"headsha","truncated":false,"tree":[
		{"path":".docz.yaml","type":"blob","sha":"cfgsha"},
		{"path":"CHANGELOG.md","type":"blob","sha":"clsha"},
		{"path":"docs/frameworks","type":"tree","sha":"dirsha"},
		{"path":"docs/frameworks/0001-intro.md","type":"blob","sha":"docsha"},
		{"path":"README.md","type":"blob","sha":"readmesha"}
	]}`
	stub := stubTransport{
		tree: tree,
		blobs: map[string]string{
			"cfgsha":    b64(cfgYAML),
			"clsha":     b64(changelo),
			"docsha":    b64(docBody),
			"readmesha": b64("# readme"),
		},
	}
	gh, err := github.NewClient(github.WithTransport(stub))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c := &Client{gh: gh}

	snap, err := c.Fetch(t.Context(), "acme", "platform")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if snap.HeadSHA != "headsha" || snap.DefaultBranch != "main" {
		t.Errorf("head/branch = %q/%q, want headsha/main", snap.HeadSHA, snap.DefaultBranch)
	}
	if string(snap.ConfigYAML) != cfgYAML {
		t.Errorf("ConfigYAML = %q, want the decoded .docz.yaml", snap.ConfigYAML)
	}
	if string(snap.ChangelogMD) != changelo || snap.ChangelogSHA != "clsha" {
		t.Errorf("changelog = %q / %q, want decoded / clsha", snap.ChangelogMD, snap.ChangelogSHA)
	}
	// No docs/index.md in the tree: the index pair stays zero with no extra
	// blob request (an unknown-sha fetch would 404 against the stub).
	if snap.IndexMD != nil || snap.IndexSHA != "" {
		t.Errorf("index = %q / %q, want absent (nil / empty)", snap.IndexMD, snap.IndexSHA)
	}
	// README.md and the tree entry are excluded; only the docz-convention doc remains.
	if len(snap.Blobs) != 1 {
		t.Fatalf("Blobs = %d, want 1 (docz-convention only)", len(snap.Blobs))
	}
	got := snap.Blobs[0]
	if got.Path != "docs/frameworks/0001-intro.md" || got.GitSHA != "docsha" || string(got.Content) != docBody {
		t.Errorf("blob = %+v, want the decoded intro doc", got)
	}
}

func TestFetchRepoIndex(t *testing.T) {
	const indexBody = "# Platform\n\nRepo home.\n"
	tests := []struct {
		name    string
		cfgYAML string
		tree    string
		blobs   map[string]string
		wantMD  string
		wantSHA string
	}{
		{
			name:    "present under default docs_dir",
			cfgYAML: "types:\n  rfc:\n    enabled: true\n",
			tree: `{"sha":"headsha","truncated":false,"tree":[
				{"path":".docz.yaml","type":"blob","sha":"cfgsha"},
				{"path":"docs/index.md","type":"blob","sha":"idxsha"}
			]}`,
			blobs:   map[string]string{"idxsha": b64(indexBody)},
			wantMD:  indexBody,
			wantSHA: "idxsha",
		},
		{
			name:    "custom docs_dir wins over the default location",
			cfgYAML: "docs_dir: notes\ntypes:\n  rfc:\n    enabled: true\n",
			tree: `{"sha":"headsha","truncated":false,"tree":[
				{"path":".docz.yaml","type":"blob","sha":"cfgsha"},
				{"path":"docs/index.md","type":"blob","sha":"decoysha"},
				{"path":"notes/index.md","type":"blob","sha":"notesidx"}
			]}`,
			blobs:   map[string]string{"notesidx": b64(indexBody)},
			wantMD:  indexBody,
			wantSHA: "notesidx",
		},
		{
			name:    "index.md as a directory is not a blob match",
			cfgYAML: "types:\n  rfc:\n    enabled: true\n",
			tree: `{"sha":"headsha","truncated":false,"tree":[
				{"path":".docz.yaml","type":"blob","sha":"cfgsha"},
				{"path":"docs/index.md","type":"tree","sha":"dirsha"}
			]}`,
			blobs: map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blobs := map[string]string{"cfgsha": b64(tt.cfgYAML)}
			maps.Copy(blobs, tt.blobs)
			gh, err := github.NewClient(github.WithTransport(stubTransport{tree: tt.tree, blobs: blobs}))
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			c := &Client{gh: gh}

			snap, err := c.Fetch(t.Context(), "acme", "platform")
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if string(snap.IndexMD) != tt.wantMD || snap.IndexSHA != tt.wantSHA {
				t.Errorf("index = %q / %q, want %q / %q",
					snap.IndexMD, snap.IndexSHA, tt.wantMD, tt.wantSHA)
			}
		})
	}
}

func TestDocsDirHint(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"explicit docs_dir", "docs_dir: notes\n", "notes"},
		{"trailing slash trimmed", "docs_dir: notes/\n", "notes"},
		{"missing key falls back to docz default", "types:\n  rfc:\n    enabled: true\n", "docs"},
		{"empty value falls back to docz default", "docs_dir: \"\"\n", "docs"},
		{"malformed yaml falls back to docz default", "\t: not yaml", "docs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := docsDirHint([]byte(tt.yaml)); got != tt.want {
				t.Errorf("docsDirHint(%q) = %q, want %q", tt.yaml, got, tt.want)
			}
		})
	}
}

func TestFetchErrorsWithoutConfig(t *testing.T) {
	tree := `{"sha":"headsha","truncated":false,"tree":[
		{"path":"docs/frameworks/0001-intro.md","type":"blob","sha":"docsha"}
	]}`
	stub := stubTransport{tree: tree, blobs: map[string]string{"docsha": b64("x")}}
	gh, err := github.NewClient(github.WithTransport(stub))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	c := &Client{gh: gh}

	if _, err := c.Fetch(t.Context(), "acme", "platform"); err == nil {
		t.Fatal("Fetch without .docz.yaml = nil error, want an error")
	}
}

func TestDecodeBlob(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
		content  string
		want     string
		wantErr  bool
	}{
		{"base64", "base64", base64.StdEncoding.EncodeToString([]byte("hello")), "hello", false},
		{"base64 wrapped", "base64", "aGVs\nbG8=", "hello", false},
		{"utf-8", "utf-8", "plain", "plain", false},
		{"empty encoding", "", "plain", "plain", false},
		{"unsupported", "latin1", "x", "", true},
		{"bad base64", "base64", "!!!!", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blob := &github.Blob{Encoding: &tt.encoding, Content: &tt.content}
			got, err := decodeBlob(blob)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("decodeBlob(%q) = nil error, want error", tt.encoding)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeBlob: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("decodeBlob = %q, want %q", got, tt.want)
			}
		})
	}
}
