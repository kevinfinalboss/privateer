package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kevinfinalboss/privateer/pkg/types"
	"github.com/rs/zerolog"
)

type Logger struct {
	logger   zerolog.Logger
	language string
	messages map[string]string
}

func New() *Logger {
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		FormatLevel: func(i interface{}) string {
			return strings.ToUpper(fmt.Sprintf("%-6s", i))
		},
	}

	logger := zerolog.New(output).
		Level(zerolog.InfoLevel).
		With().
		Timestamp().
		Caller().
		Logger()

	l := &Logger{
		logger:   logger,
		language: "pt-BR",
	}
	l.loadMessages()
	return l
}

func NewWithConfig(cfg *types.Config) *Logger {
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		FormatLevel: func(i interface{}) string {
			return strings.ToUpper(fmt.Sprintf("%-6s", i))
		},
	}

	level := parseLogLevel(cfg.Settings.LogLevel)

	logger := zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Caller().
		Logger()

	l := &Logger{
		logger:   logger,
		language: cfg.Settings.Language,
	}
	l.loadMessages()
	return l
}

func parseLogLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

func (l *Logger) loadMessages() {
	messages, err := loadLocaleMessages(l.language)
	if err != nil {
		messages = getEmbeddedMessages(l.language)
	}
	l.messages = messages
}

func (l *Logger) GetMessage(key string) string {
	return l.getMessage(key)
}

func (l *Logger) getMessage(key string) string {
	if message, exists := l.messages[key]; exists {
		return message
	}

	fallbackMessages, _ := loadLocaleMessages("en-US")
	if message, exists := fallbackMessages[key]; exists {
		return message
	}

	return key
}

func (l *Logger) Debug(key string) *zerolog.Event {
	return l.logger.Debug().Str("message", l.getMessage(key))
}

func (l *Logger) Info(key string) *zerolog.Event {
	return l.logger.Info().Str("message", l.getMessage(key))
}

func (l *Logger) Warn(key string) *zerolog.Event {
	return l.logger.Warn().Str("message", l.getMessage(key))
}

func (l *Logger) Error(key string) *zerolog.Event {
	return l.logger.Error().Str("message", l.getMessage(key))
}

func (l *Logger) Fatal(key string) *zerolog.Event {
	return l.logger.Fatal().Str("message", l.getMessage(key))
}

func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	ctx := l.logger.With()
	for k, v := range fields {
		ctx = ctx.Interface(k, v)
	}

	return &Logger{
		logger:   ctx.Logger(),
		language: l.language,
		messages: l.messages,
	}
}

func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{
		logger:   l.logger.With().Interface(key, value).Logger(),
		language: l.language,
		messages: l.messages,
	}
}
