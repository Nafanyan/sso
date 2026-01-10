package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// prettyHandler - кастомный handler с цветным выводом для local среды
type prettyHandler struct {
	opts *slog.HandlerOptions
	w    io.Writer
}

// NewPrettyHandler создает новый pretty handler с цветным выводом
func NewPrettyHandler(w io.Writer, opts *slog.HandlerOptions) *prettyHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &prettyHandler{
		opts: opts,
		w:    w,
	}
}

// ANSI escape коды для цветов
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
	// Цвет для уровня логирования
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

	// Форматирование времени
	timeStr := record.Time.Format("15:04:05")

	// Собираем атрибуты
	var attrs []string
	record.Attrs(func(a slog.Attr) bool {
		key := a.Key
		value := a.Value.String()

		// Цвета для разных типов полей
		var attrStr string
		if key == "op" {
			attrStr = fmt.Sprintf("%s[%s]%s", purple, value, reset)
		} else if key == "error" {
			attrStr = fmt.Sprintf("%s%s=%s%s", red, key, value, reset)
		} else {
			attrStr = blue + key + reset + "=" + gray + value + reset
		}
		attrs = append(attrs, attrStr)
		return true
	})

	// Формируем финальную строку
	levelStr := record.Level.String()
	if len(levelStr) < 5 {
		levelStr += " "
	}

	// Время + уровень с иконкой + сообщение
	msg := gray + timeStr + reset + " " +
		levelColor + bold + levelStr + levelIcon + reset + " " +
		bold + record.Message + reset + " "

	// Добавляем атрибуты
	for _, attr := range attrs {
		msg += attr + " "
	}

	msg += "\n"

	_, err := h.w.Write([]byte(msg))
	return err
}

func (h *prettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Упрощенная реализация - возвращаем тот же handler
	return h
}

func (h *prettyHandler) WithGroup(name string) slog.Handler {
	// Упрощенная реализация - возвращаем тот же handler
	return h
}
