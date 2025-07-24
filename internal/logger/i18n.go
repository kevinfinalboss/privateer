package logger

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type LocaleMessages struct {
	Messages map[string]string `yaml:"messages"`
}

func loadLocaleMessages(language string) (map[string]string, error) {
	filename := language + ".yaml"

	localeFile := filepath.Join("locales", filename)

	data, err := os.ReadFile(localeFile)
	if err != nil {
		fallbackFile := filepath.Join("locales", "en-US.yaml")
		data, err = os.ReadFile(fallbackFile)
		if err != nil {
			return getEmbeddedMessages("en-US"), nil
		}
	}

	var locale LocaleMessages
	if err := yaml.Unmarshal(data, &locale); err != nil {
		return getEmbeddedMessages(language), nil
	}

	return locale.Messages, nil
}

func getEmbeddedMessages(language string) map[string]string {
	switch strings.ToLower(language) {
	case "pt-br":
		return map[string]string{
			"app_started":           "Privateer iniciado",
			"connecting_k8s":        "Conectando ao cluster Kubernetes",
			"k8s_connected":         "Conectado ao cluster Kubernetes",
			"k8s_connection_failed": "Falha ao conectar com o cluster",
			"scanning_cluster":      "Escaneando cluster",
			"scanning_namespace":    "Escaneando namespace",
			"images_found":          "Imagens encontradas",
			"public_image_found":    "Imagem pública encontrada",
			"config_not_found":      "Arquivo de configuração não encontrado",
			"config_loaded":         "Configuração carregada",
			"config_created":        "Arquivo de configuração criado",
			"config_already_exists": "Arquivo de configuração já existe",
			"operation_completed":   "Operação concluída",
			"operation_failed":      "Operação falhou",
		}
	case "es-es":
		return map[string]string{
			"app_started":           "Privateer iniciado",
			"connecting_k8s":        "Conectando al cluster de Kubernetes",
			"k8s_connected":         "Conectado al cluster de Kubernetes",
			"k8s_connection_failed": "Error al conectar con el cluster",
			"scanning_cluster":      "Escaneando cluster",
			"scanning_namespace":    "Escaneando namespace",
			"images_found":          "Imágenes encontradas",
			"public_image_found":    "Imagen pública encontrada",
			"config_not_found":      "Archivo de configuración no encontrado",
			"config_loaded":         "Configuración cargada",
			"config_created":        "Archivo de configuración creado",
			"config_already_exists": "Archivo de configuración ya existe",
			"operation_completed":   "Operación completada",
			"operation_failed":      "Operación falló",
		}
	default:
		return map[string]string{
			"app_started":           "Privateer started",
			"connecting_k8s":        "Connecting to Kubernetes cluster",
			"k8s_connected":         "Connected to Kubernetes cluster",
			"k8s_connection_failed": "Failed to connect to cluster",
			"scanning_cluster":      "Scanning cluster",
			"scanning_namespace":    "Scanning namespace",
			"images_found":          "Images found",
			"public_image_found":    "Public image found",
			"config_not_found":      "Configuration file not found",
			"config_loaded":         "Configuration loaded",
			"config_created":        "Configuration file created",
			"config_already_exists": "Configuration file already exists",
			"operation_completed":   "Operation completed",
			"operation_failed":      "Operation failed",
		}
	}
}
