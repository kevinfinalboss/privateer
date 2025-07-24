package registry

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

type Registry interface {
	Login(ctx context.Context) error
	Push(ctx context.Context, image *types.ImageInfo, targetTag string) error
	Pull(ctx context.Context, imageName string) error
	Copy(ctx context.Context, sourceImage, targetImage string) error
	GetType() string
	GetName() string
	IsHealthy(ctx context.Context) error
}

type BaseRegistry struct {
	Name     string
	Type     string
	Logger   *logger.Logger
	Username string
	Password string
	URL      string
	Insecure bool
}

func (r *BaseRegistry) GetType() string {
	return r.Type
}

func (r *BaseRegistry) GetName() string {
	return r.Name
}

type RegistryConfig = types.RegistryConfig

type Manager struct {
	registries map[string]Registry
	logger     *logger.Logger
}

func NewManager(logger *logger.Logger) *Manager {
	return &Manager{
		registries: make(map[string]Registry),
		logger:     logger,
	}
}

func (m *Manager) AddRegistry(config *types.RegistryConfig) error {
	var registry Registry
	var err error

	switch config.Type {
	case "docker":
		registry, err = NewDockerRegistry(config, m.logger)
	case "harbor":
		registry, err = NewHarborRegistry(config, m.logger)
	case "ecr":
		registry, err = NewECRRegistry(config, m.logger)
	case "ghcr":
		registry, err = NewGHCRRegistry(config, m.logger)
	default:
		return fmt.Errorf("tipo de registry não suportado: %s", config.Type)
	}

	if err != nil {
		return fmt.Errorf("falha ao criar registry %s: %w", config.Name, err)
	}

	m.registries[config.Name] = registry
	m.logger.Info("registry_added").
		Str("name", config.Name).
		Str("type", config.Type).
		Send()

	return nil
}

func (m *Manager) GetRegistry(name string) (Registry, error) {
	registry, exists := m.registries[name]
	if !exists {
		return nil, fmt.Errorf("registry não encontrado: %s", name)
	}
	return registry, nil
}

func (m *Manager) ListRegistries() []string {
	var names []string
	for name := range m.registries {
		names = append(names, name)
	}
	return names
}

func (m *Manager) HealthCheck(ctx context.Context) error {
	for name, registry := range m.registries {
		if err := registry.IsHealthy(ctx); err != nil {
			m.logger.Warn("registry_unhealthy").
				Str("name", name).
				Err(err).
				Send()
			return fmt.Errorf("registry %s não está saudável: %w", name, err)
		}
		m.logger.Debug("registry_healthy").
			Str("name", name).
			Send()
	}
	return nil
}

func createHTTPClient(insecure bool) *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecure,
			},
		},
	}
}
