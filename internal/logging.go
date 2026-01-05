package internal

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// TraceContextHandler wraps a slog.Handler to inject trace context into log records
type TraceContextHandler struct {
	handler slog.Handler
}

// NewTraceContextHandler creates a new handler that adds trace context to log records
func NewTraceContextHandler(handler slog.Handler) *TraceContextHandler {
	return &TraceContextHandler{handler: handler}
}

// Enabled reports whether the handler handles records at the given level
func (h *TraceContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle handles the record by adding trace context and passing it to the wrapped handler
func (h *TraceContextHandler) Handle(ctx context.Context, record slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		spanContext := span.SpanContext()
		if spanContext.IsValid() {
			record.AddAttrs(
				slog.String("trace_id", spanContext.TraceID().String()),
				slog.String("span_id", spanContext.SpanID().String()),
			)
		}
	}
	return h.handler.Handle(ctx, record)
}

// WithAttrs returns a new handler with the given attributes
func (h *TraceContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceContextHandler{handler: h.handler.WithAttrs(attrs)}
}

// WithGroup returns a new handler with the given group
func (h *TraceContextHandler) WithGroup(name string) slog.Handler {
	return &TraceContextHandler{handler: h.handler.WithGroup(name)}
}
