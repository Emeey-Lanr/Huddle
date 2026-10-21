package audit

import (
	"context"
	"log/slog"
	"os"
	"time"
)

var Logger *slog.Logger

func InitAuditLogger() {
	// Production JSON audit logger using Go standard library log/slog
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	Logger = slog.New(handler).With("app", "huddle-api")
	slog.SetDefault(Logger)
}

func LogEvent(ctx context.Context, event string, actorID string, targetID string, status string, metadata map[string]interface{}) {
	if Logger == nil {
		InitAuditLogger()
	}

	attrs := []slog.Attr{
		slog.String("event", event),
		slog.String("actor_id", actorID),
		slog.String("target_id", targetID),
		slog.String("status", status),
		slog.String("timestamp", time.Now().UTC().Format(time.RFC3339Nano)),
	}

	if len(metadata) > 0 {
		attrs = append(attrs, slog.Any("metadata", metadata))
	}

	Logger.LogAttrs(ctx, slog.LevelInfo, event, attrs...)
}
