package oauth

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// TestMetadataDocuments pins the three documents against goldens, with the
// key's thumbprint fixed by a fixed key.
func TestMetadataDocuments(t *testing.T) {
	t.Parallel()

	signer, err := NewSigner(testIssuer, testAudience, fixedKey(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	meta := NewMetadata(testIssuer, testAudience, []string{"team:read", "public:read", "docz:read", "team:read"}, signer.JWKS())

	r := chi.NewRouter()
	meta.Mount(r)

	for _, tc := range []struct{ name, path string }{
		{"protected-resource", ProtectedResourcePath},
		{"authorization-server", AuthorizationServerPath},
		{"jwks", JWKSPath},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, http.NoBody))
			if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("GET %s = %d %s", tc.path, rec.Code, rec.Header().Get("Content-Type"))
			}

			var pretty bytes.Buffer
			if err := json.Indent(&pretty, rec.Body.Bytes(), "", "  "); err != nil {
				t.Fatal(err)
			}
			pretty.WriteByte('\n')

			golden := filepath.Join("testdata", "metadata", tc.name+".golden.json")
			if *update {
				if err := os.WriteFile(golden, pretty.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(want, pretty.Bytes()) {
				t.Errorf("%s differs from golden:\n%s", tc.path, pretty.String())
			}
		})
	}
}

func TestScopesSupported(t *testing.T) {
	t.Parallel()

	got := scopesSupported([]string{"z:read", "a:read", "public:read", "a:read"})
	want := []string{"public:read", "a:read", "z:read"}
	if len(got) != len(want) {
		t.Fatalf("scopesSupported = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("scopesSupported = %v, want %v", got, want)
		}
	}
}
