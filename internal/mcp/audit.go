package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/donaldgifford/docz/v2/internal/session"
	"github.com/donaldgifford/docz/v2/internal/telemetry"
)

// tracer is the instrumentation scope for the mcp.<method> spans.
var tracer = otel.Tracer("github.com/donaldgifford/docz/v2/internal/mcp")

// The MCP methods the middleware singles out, and the request headers the
// 2026-07-28 revision carries beside the body.
const (
	methodCallTool     = "tools/call"
	methodReadResource = "resources/read"
	headerMcpName      = "Mcp-Name"
	metaTraceparent    = "traceparent"
	metaTracestate     = "tracestate"
)

// The outcome labels.
const (
	statusOK    = "ok"
	statusError = "error"
)

// auditArgs are the only arguments the audit line reads: the repository
// and the document named. Never a search query, which is a person's
// words, and never a document's text (DESIGN-0023 §8).
type auditArgs struct {
	Repo string `json:"repo"`
	ID   string `json:"id"`
}

// observe is the receiving middleware: a child span mcp.<method> under
// the request's span (or under the traceparent the call's _meta carries
// when the HTTP headers carried none), the two MCP metrics labelled from
// the Mcp-Name header, and the mcp call audit line for a tool call and a
// resource read.
func observe(logger *slog.Logger) sdk.Middleware {
	return func(next sdk.MethodHandler) sdk.MethodHandler {
		return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
			ctx = parentFromMeta(ctx, req)
			tool := toolName(method, req)
			ctx, span := tracer.Start(ctx, "mcp."+method, trace.WithAttributes(
				attribute.String("mcp.method", method),
				attribute.String("mcp.tool", tool),
			))
			defer span.End()

			start := time.Now()
			result, err := next(ctx, method, req)
			dur := time.Since(start)

			status := statusOK
			if err != nil || isError(result) {
				status = statusError
				span.SetStatus(codes.Error, status)
			}
			telemetry.ObserveMCPCall(tool, status, dur)

			if method == methodCallTool || method == methodReadResource {
				audit(ctx, logger, req, tool, status, dur)
			}

			return result, err
		}
	}
}

// audit writes the mcp call line: who, how, what, and the outcome.
func audit(ctx context.Context, logger *slog.Logger, req sdk.Request, tool, status string, dur time.Duration) {
	sess, _ := session.FromContext(ctx)
	repo, docID := named(req)
	logger.LogAttrs(ctx, slog.LevelInfo, "mcp call",
		slog.String("principal", sess.PrincipalKey()),
		slog.String("via", sess.Via.String()),
		slog.String("client_id", sess.ClientID),
		slog.String("issuer", sess.Issuer),
		slog.Any("scopes", sess.Scopes),
		slog.String("tool", tool),
		slog.String("repo", repo),
		slog.String("doc_id", docID),
		slog.String("status", status),
		slog.Duration("duration", dur),
	)
}

// named reads the repository and the document a request names.
func named(req sdk.Request) (repo, docID string) {
	switch p := req.GetParams().(type) {
	case *sdk.CallToolParamsRaw:
		var args auditArgs
		if len(p.Arguments) > 0 {
			if err := json.Unmarshal(p.Arguments, &args); err != nil {
				return "", ""
			}
		}

		return args.Repo, args.ID
	case *sdk.ReadResourceParams:
		owner, name, _, id, ok := parseResourceURI(p.URI)
		if !ok {
			return "", ""
		}

		return owner + "/" + name, id
	default:
		return "", ""
	}
}

// toolName is the metric's tool label: the Mcp-Name header's value when
// the transport carried one, else the tool named in the body for a tool
// call, else the method, so cardinality is the tool set plus the methods.
func toolName(method string, req sdk.Request) string {
	if extra := req.GetExtra(); extra != nil && extra.Header != nil {
		if name := extra.Header.Get(headerMcpName); name != "" {
			return name
		}
	}
	if p, ok := req.GetParams().(*sdk.CallToolParamsRaw); ok && p.Name != "" {
		return p.Name
	}

	return method
}

// parentFromMeta extracts the traceparent the call's _meta carries, but
// only when ctx holds no span already: the HTTP headers win when present.
func parentFromMeta(ctx context.Context, req sdk.Request) context.Context {
	if trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	params := req.GetParams()
	if params == nil {
		return ctx
	}
	meta := params.GetMeta()
	tp, ok := meta[metaTraceparent].(string)
	if !ok || tp == "" {
		return ctx
	}
	carrier := propagation.MapCarrier{metaTraceparent: tp}
	if ts, ok := meta[metaTracestate].(string); ok && ts != "" {
		carrier[metaTracestate] = ts
	}

	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// isError reports a tool result that carries isError.
func isError(result sdk.Result) bool {
	r, ok := result.(*sdk.CallToolResult)

	return ok && r.IsError
}
