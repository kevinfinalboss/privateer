package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kevinfinalboss/privateer/internal/config"
	"github.com/rs/zerolog"
)

type Logger struct {
	logger   zerolog.Logger
	language string
	messages map[string]map[string]string
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
		messages: make(map[string]map[string]string),
	}
	l.loadMessages()
	return l
}

func NewWithConfig(cfg *config.Config) *Logger {
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
		messages: make(map[string]map[string]string),
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
	l.messages["pt-BR"] = map[string]string{
		"app_started":             "Privateer iniciado",
		"app_version":             "Versão do Privateer",
		"connecting_k8s":          "Conectando ao cluster Kubernetes",
		"k8s_connected":           "Conectado ao cluster Kubernetes",
		"k8s_connection_failed":   "Falha ao conectar com o cluster",
		"scanning_cluster":        "Escaneando cluster",
		"scanning_namespace":      "Escaneando namespace",
		"images_found":            "Imagens encontradas",
		"public_images_found":     "Imagens públicas encontradas",
		"application_failed":      "Aplicação falhou",
		"config_loaded":           "Configuração carregada",
		"config_not_found":        "Arquivo de configuração não encontrado",
		"operation_completed":     "Operação concluída",
		"operation_failed":        "Operação falhou",
		"registry_connection":     "Conectando ao registry",
		"registry_connected":      "Conectado ao registry",
		"image_pulled":            "Imagem baixada",
		"image_tagged":            "Imagem retagueada",
		"image_pushed":            "Imagem enviada",
		"public_image_found":      "Imagem pública encontrada",
		"resource_scanned":        "Recurso escaneado",
		"registry_ignored":        "Registry ignorado",
		"custom_private_registry": "Registry privado customizado",
		"custom_public_registry":  "Registry público customizado",
		"config_created":          "Arquivo de configuração criado",
		"config_already_exists":   "Arquivo de configuração já existe",
	}

	l.messages["en-US"] = map[string]string{
		"app_started":             "Privateer started",
		"app_version":             "Privateer version",
		"connecting_k8s":          "Connecting to Kubernetes cluster",
		"k8s_connected":           "Connected to Kubernetes cluster",
		"k8s_connection_failed":   "Failed to connect to cluster",
		"scanning_cluster":        "Scanning cluster",
		"scanning_namespace":      "Scanning namespace",
		"images_found":            "Images found",
		"public_images_found":     "Public images found",
		"application_failed":      "Application failed",
		"config_loaded":           "Configuration loaded",
		"config_not_found":        "Configuration file not found",
		"operation_completed":     "Operation completed",
		"operation_failed":        "Operation failed",
		"registry_connection":     "Connecting to registry",
		"registry_connected":      "Connected to registry",
		"image_pulled":            "Image pulled",
		"image_tagged":            "Image tagged",
		"image_pushed":            "Image pushed",
		"public_image_found":      "Public image found",
		"resource_scanned":        "Resource scanned",
		"registry_ignored":        "Registry ignored",
		"custom_private_registry": "Custom private registry",
		"custom_public_registry":  "Custom public registry",
		"config_created":          "Configuration file created",
		"config_already_exists":   "Configuration file already exists",
	}

	l.messages["es-ES"] = map[string]string{
		"app_started":           "Privateer iniciado",
		"app_version":           "Versión de Privateer",
		"connecting_k8s":        "Conectando al cluster de Kubernetes",
		"k8s_connected":         "Conectado al cluster de Kubernetes",
		"k8s_connection_failed": "Error al conectar con el cluster",
		"scanning_cluster":      "Escaneando cluster",
		"scanning_namespace":    "Escaneando namespace",
		"images_found":          "Imágenes encontradas",
		"public_images_found":   "Imágenes públicas encontradas",
		"application_failed":    "Aplicación falló",
		"config_loaded":         "Configuración cargada",
		"config_not_found":      "Archivo de configuración no encontrado",
		"operation_completed":   "Operación completada",
		"operation_failed":      "Operación falló",
		"registry_connection":   "Conectando al registry",
		"registry_connected":    "Conectado al registry",
		"image_pulled":          "Imagen descargada",
		"image_tagged":          "Imagen etiquetada",
		"image_pushed":          "Imagen enviada",
		"public_image_found":    "Imagen pública encontrada",
		"resource_scanned":      "Recurso escaneado",
	}
}

func (l *Logger) getMessage(key string) string {
	if messages, exists := l.messages[l.language]; exists {
		if message, exists := messages[key]; exists {
			return message
		}
	}

	if messages, exists := l.messages["en-US"]; exists {
		if message, exists := messages[key]; exists {
			return message
		}
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
