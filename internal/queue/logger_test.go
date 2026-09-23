package queue

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
)

// recordedLog is one captured slog record, flattened for assertions.
type recordedLog struct {
	level slog.Level
	msg   string
	attrs map[string]string
}

// recorder is a slog.Handler that keeps every record in memory. It exists
// because the Phase 1 contract is about what reaches the logs, so the tests
// assert on emitted records rather than on return values.
type recorder struct {
	records []recordedLog
}

func (*recorder) Enabled(context.Context, slog.Level) bool { return true }

// The slog.Handler interface fixes this signature, so the by-value slog.Record
// cannot be taken by pointer.
//
//nolint:gocritic // hugeParam: signature mandated by slog.Handler.
func (r *recorder) Handle(_ context.Context, rec slog.Record) error {
	entry := recordedLog{level: rec.Level, msg: rec.Message, attrs: map[string]string{}}
	rec.Attrs(func(a slog.Attr) bool {
		entry.attrs[a.Key] = a.Value.String()
		return true
	})
	r.records = append(r.records, entry)
	return nil
}

func (r *recorder) WithAttrs([]slog.Attr) slog.Handler { return r }
func (r *recorder) WithGroup(string) slog.Handler      { return r }

// only returns the single captured record, failing when the count differs.
func (r *recorder) only(t *testing.T) recordedLog {
	t.Helper()
	if len(r.records) != 1 {
		t.Fatalf("want exactly 1 log record, got %d: %+v", len(r.records), r.records)
	}
	return r.records[0]
}

// captureLogs installs a recording handler as the slog default for the test and
// restores the previous default afterwards.
func captureLogs(t *testing.T) *recorder {
	t.Helper()
	rec := &recorder{}
	prev := slog.Default()
	slog.SetDefault(slog.New(rec))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return rec
}

func TestAsynqLoggerMapsLevels(t *testing.T) {
	tests := []struct {
		name string
		call func(l *asynqLogger)
		want slog.Level
	}{
		{"debug", func(l *asynqLogger) { l.Debug("m") }, slog.LevelDebug},
		{"info", func(l *asynqLogger) { l.Info("m") }, slog.LevelInfo},
		{"warn", func(l *asynqLogger) { l.Warn("m") }, slog.LevelWarn},
		{"error", func(l *asynqLogger) { l.Error("m") }, slog.LevelError},
		// Fatal must not exit the process; it degrades to Error.
		{"fatal", func(l *asynqLogger) { l.Fatal("m") }, slog.LevelError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &recorder{}
			tt.call(newAsynqLogger(slog.New(rec)))

			got := rec.only(t)
			if got.level != tt.want {
				t.Errorf("level = %v, want %v", got.level, tt.want)
			}
			if got.msg != "m" {
				t.Errorf("msg = %q, want %q", got.msg, "m")
			}
			if got.attrs["component"] != "asynq" {
				t.Errorf("component = %q, want %q", got.attrs["component"], "asynq")
			}
		})
	}
}

func TestAsynqLoggerJoinsArgs(t *testing.T) {
	rec := &recorder{}
	newAsynqLogger(slog.New(rec)).Error("Dequeue error: ", errors.New("redis down"))

	if got := rec.only(t).msg; !strings.Contains(got, "redis down") {
		t.Errorf("msg = %q, want it to contain the joined args", got)
	}
}

func TestAsynqLoggerDefaultsToSlogDefault(t *testing.T) {
	rec := captureLogs(t)
	newAsynqLogger(nil).Warn("fallback")

	if got := rec.only(t).msg; got != "fallback" {
		t.Errorf("msg = %q, want %q", got, "fallback")
	}
}

func TestAsynqLogLevelFollowsSlog(t *testing.T) {
	tests := []struct {
		name string
		slog slog.Level
		want asynq.LogLevel
	}{
		{"debug", slog.LevelDebug, asynq.DebugLevel},
		{"info", slog.LevelInfo, asynq.InfoLevel},
		{"warn", slog.LevelWarn, asynq.WarnLevel},
		{"error", slog.LevelError, asynq.ErrorLevel},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(nopWriter{}, &slog.HandlerOptions{Level: tt.slog}))
			if got := asynqLogLevel(logger); got != tt.want {
				t.Errorf("asynqLogLevel = %v, want %v", got, tt.want)
			}
		})
	}
}

// nopWriter discards handler output; these tests assert on the derived level,
// not on rendered text.
type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestLogIngestFailureReportsRepoAndError(t *testing.T) {
	rec := captureLogs(t)
	job := &IngestJob{InstallationID: 7, Owner: "acme", Name: "docs", Reason: "push"}

	logIngestFailure(context.Background(), makeTask(t, job), errors.New("boom"))

	got := rec.only(t)
	if got.level != slog.LevelError {
		t.Errorf("level = %v, want Error", got.level)
	}
	if got.attrs["repo"] != "acme/docs" {
		t.Errorf("repo = %q, want %q", got.attrs["repo"], "acme/docs")
	}
	if got.attrs["reason"] != "push" {
		t.Errorf("reason = %q, want %q", got.attrs["reason"], "push")
	}
	if !strings.Contains(got.attrs["err"], "boom") {
		t.Errorf("err = %q, want it to contain %q", got.attrs["err"], "boom")
	}
}

// A malformed payload is itself a reported failure, so the handler must still
// log the real error rather than being derailed by the decode failure.
func TestLogIngestFailureToleratesMalformedPayload(t *testing.T) {
	rec := captureLogs(t)
	task := asynq.NewTask(TaskTypeIngest, []byte("{not json"))

	logIngestFailure(context.Background(), task, errors.New("boom"))

	got := rec.only(t)
	if !strings.Contains(got.attrs["err"], "boom") {
		t.Errorf("err = %q, want it to contain %q", got.attrs["err"], "boom")
	}
	if _, ok := got.attrs["repo"]; ok {
		t.Errorf("repo attr = %q, want it omitted for an undecodable payload", got.attrs["repo"])
	}
}
