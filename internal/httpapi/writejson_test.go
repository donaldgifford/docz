package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// errLogCounter counts error-level records.
type errLogCounter struct{ errors int }

func (*errLogCounter) Enabled(context.Context, slog.Level) bool { return true }

//nolint:gocritic // hugeParam: signature mandated by slog.Handler.
func (c *errLogCounter) Handle(_ context.Context, rec slog.Record) error {
	if rec.Level == slog.LevelError {
		c.errors++
	}
	return nil
}

func (c *errLogCounter) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *errLogCounter) WithGroup(string) slog.Handler      { return c }

// unmarshalable fails json.Marshal, standing in for a DTO carrying invalid
// json.RawMessage from the database.
type unmarshalable struct {
	Ch chan int `json:"ch"`
}

// Every /api/v1 response funnels through writeJSON, so a marshal failure must
// not become an opaque 500 with nothing in the logs (INV-0007 F7.4).
func TestWriteJSONLogsMarshalFailure(t *testing.T) {
	counter := &errLogCounter{}
	prev := slog.Default()
	slog.SetDefault(slog.New(counter))
	t.Cleanup(func() { slog.SetDefault(prev) })

	rec := httptest.NewRecorder()
	writeJSON(rec, unmarshalable{})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if counter.errors != 1 {
		t.Errorf("error records = %d, want 1", counter.errors)
	}
}
