package logger

import (
	"context"
	"io"
	"log/slog"
)

type prettyHandler struct {
	opts  *slog.HandlerOptions
	w     io.Writer
	attrs []slog.Attr // атрибуты из With()
}

func NewPrettyHandler(w io.Writer, opts *slog.HandlerOptions) *prettyHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &prettyHandler{
		opts: opts,
		w:    w,
	}
}

const (
	reset  = "\033[0m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	purple = "\033[35m"
	cyan   = "\033[36m"
	gray   = "\033[90m"
	bold   = "\033[1m"
)

func (h *prettyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.opts.Level != nil {
		return level >= h.opts.Level.Level()
	}
	return true
}

func (h *prettyHandler) Handle(ctx context.Context, record slog.Record) error {
	var levelColor, levelIcon string
	switch record.Level {
	case slog.LevelDebug:
		levelColor = gray
		levelIcon = "🔍"
	case slog.LevelInfo:
		levelColor = cyan
		levelIcon = "ℹ️ "
	case slog.LevelWarn:
		levelColor = yellow
		levelIcon = "⚠️ "
	case slog.LevelError:
		levelColor = red
		levelIcon = "❌"
	default:
		levelColor = reset
		levelIcon = "  "
	}

	timeStr := record.Time.Format("15:04:05")

	formatAttr := func(a slog.Attr) string {
		return blue + a.Key + reset + "=" + gray + a.Value.String() + reset
	}

	var attrs []string
	for _, a := range h.attrs {
		attrs = append(attrs, formatAttr(a))
	}
	record.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, formatAttr(a))
		return true
	})

	levelStr := record.Level.String()
	if len(levelStr) < 5 {
		levelStr += " "
	}

	msg := gray + timeStr + reset + " " +
		levelColor + bold + levelStr + levelIcon + reset + " " +
		bold + record.Message + reset + " "

	for _, attr := range attrs {
		msg += attr + " "
	}

	msg += "\n"

	_, err := h.w.Write([]byte(msg))
	return err
}

func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	newAttrs = append(newAttrs, h.attrs...)
	newAttrs = append(newAttrs, attrs...)
	return &prettyHandler{opts: h.opts, w: h.w, attrs: newAttrs}
}

func (h *prettyHandler) WithGroup(name string) slog.Handler {
	return h
}
