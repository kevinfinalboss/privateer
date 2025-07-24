package registry

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
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
	HasImage(ctx context.Context, imageName string) (bool, error)
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

type Manager struct {
	registries map[string]Registry
	logger     *logger.Logger
	mutex      sync.RWMutex
}

func NewManager(logger *logger.Logger) *Manager {
	return &Manager{
		registries: make(map[string]Registry),
		logger:     logger,
	}
}

func (m *Manager) AddRegistry(config *types.RegistryConfig) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !config.Enabled {
		m.logger.Debug("registry_disabled").
			Str("name", config.Name).
			Send()
		return nil
	}

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
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	registry, exists := m.registries[name]
	if !exists {
		return nil, fmt.Errorf("registry %s não encontrado", name)
	}

	return registry, nil
}

func (m *Manager) ListRegistries() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var names []string
	for name := range m.registries {
		names = append(names, name)
	}

	return names
}

func (m *Manager) GetEnabledRegistries() []Registry {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var enabled []Registry
	for _, registry := range m.registries {
		enabled = append(enabled, registry)
	}

	return enabled
}

func (m *Manager) HealthCheck(ctx context.Context) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var errors []error

	for name, registry := range m.registries {
		m.logger.Debug("registry_health_check").
			Str("name", name).
			Send()

		if err := registry.IsHealthy(ctx); err != nil {
			m.logger.Error("registry_health_check_failed").
				Str("name", name).
				Err(err).
				Send()
			errors = append(errors, fmt.Errorf("registry %s: %w", name, err))
		} else {
			m.logger.Info("registry_health_check_success").
				Str("name", name).
				Send()
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("falhas no health check: %v", errors)
	}

	return nil
}

func (m *Manager) CheckImageExists(ctx context.Context, imageName string) (map[string]bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	results := make(map[string]bool)

	for name, registry := range m.registries {
		exists, err := registry.HasImage(ctx, imageName)
		if err != nil {
			m.logger.Warn("image_check_failed").
				Str("registry", name).
				Str("image", imageName).
				Err(err).
				Send()
			continue
		}

		results[name] = exists

		if exists {
			m.logger.Debug("image_exists").
				Str("registry", name).
				Str("image", imageName).
				Send()
		}
	}

	return results, nil
}

func (m *Manager) ValidateImageDuplication(ctx context.Context, targetImage string) error {
	existsMap, err := m.CheckImageExists(ctx, targetImage)
	if err != nil {
		return fmt.Errorf("falha ao verificar duplicação de imagem: %w", err)
	}

	var duplicatedRegistries []string
	for registryName, exists := range existsMap {
		if exists {
			duplicatedRegistries = append(duplicatedRegistries, registryName)
		}
	}

	if len(duplicatedRegistries) > 0 {
		m.logger.Warn("image_already_exists").
			Str("image", targetImage).
			Strs("registries", duplicatedRegistries).
			Send()
		return fmt.Errorf("imagem %s já existe nos registries: %v", targetImage, duplicatedRegistries)
	}

	return nil
}

func (m *Manager) GetRegistryCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.registries)
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
