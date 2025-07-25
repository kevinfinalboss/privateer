package registry

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
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

func (m *Manager) ValidateImagesBatch(ctx context.Context, images []*types.ImageInfo, config *types.Config) (map[string]string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	m.logger.Info("validating_images_batch").
		Int("images", len(images)).
		Int("registries", len(m.registries)).
		Send()

	validatedMap := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	semaphore := make(chan struct{}, 10)

	for _, image := range images {
		wg.Add(1)
		go func(img *types.ImageInfo) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			for _, registry := range m.registries {
				targetImage := m.generateTargetImageName(img, registry, config)

				m.logger.Debug("batch_validating_image").
					Str("public", img.Image).
					Str("target", targetImage).
					Str("registry", registry.GetName()).
					Send()

				exists, err := registry.HasImage(ctx, targetImage)
				if err != nil {
					m.logger.Warn("batch_validation_failed").
						Str("image", targetImage).
						Str("registry", registry.GetName()).
						Err(err).
						Send()
					continue
				}

				if exists {
					mu.Lock()
					validatedMap[img.Image] = targetImage
					mu.Unlock()

					m.logger.Info("batch_image_validated").
						Str("public", img.Image).
						Str("private", targetImage).
						Str("registry", registry.GetName()).
						Send()
					return
				}
			}

			m.logger.Debug("batch_image_not_found").
				Str("public_image", img.Image).
				Send()
		}(image)
	}

	wg.Wait()

	m.logger.Info("batch_validation_completed").
		Int("validated", len(validatedMap)).
		Int("total", len(images)).
		Send()

	return validatedMap, nil
}

func (m *Manager) FindImageInRegistries(ctx context.Context, publicImage *types.ImageInfo, config *types.Config) (string, string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	m.logger.Debug("finding_image_in_registries").
		Str("public_image", publicImage.Image).
		Send()

	for _, registry := range m.registries {
		targetImage := m.generateTargetImageName(publicImage, registry, config)

		exists, err := registry.HasImage(ctx, targetImage)
		if err != nil {
			m.logger.Warn("registry_check_failed").
				Str("registry", registry.GetName()).
				Str("image", targetImage).
				Err(err).
				Send()
			continue
		}

		if exists {
			m.logger.Info("image_found_in_registry").
				Str("public", publicImage.Image).
				Str("private", targetImage).
				Str("registry", registry.GetName()).
				Send()
			return targetImage, registry.GetName(), nil
		}
	}

	return "", "", fmt.Errorf("imagem %s não encontrada em nenhum registry", publicImage.Image)
}

func (m *Manager) generateTargetImageName(image *types.ImageInfo, reg Registry, config *types.Config) string {
	parsed := parseImageName(image.Image)
	targetRepository := parsed.FullRepository
	targetTag := parsed.Tag

	if parsed.Digest != "" {
		targetTag = fmt.Sprintf("%s@%s", targetTag, parsed.Digest)
	}

	switch reg.GetType() {
	case "docker":
		registryURL := m.getRegistryURL(reg.GetName(), config)
		return fmt.Sprintf("%s/%s:%s", registryURL, targetRepository, targetTag)

	case "harbor":
		registryURL := m.getRegistryURL(reg.GetName(), config)
		project := m.getHarborProject(reg.GetName(), config)
		return fmt.Sprintf("%s/%s/%s:%s", registryURL, project, targetRepository, targetTag)

	case "ecr":
		ecrURL := m.getECRURL(reg.GetName(), config)
		return fmt.Sprintf("%s/%s:%s", ecrURL, targetRepository, targetTag)

	case "ghcr":
		organization := m.getGHCROrganization(reg.GetName(), config)
		return fmt.Sprintf("ghcr.io/%s/%s:%s", organization, targetRepository, targetTag)
	}

	return fmt.Sprintf("%s/%s:%s", reg.GetName(), targetRepository, targetTag)
}

func (m *Manager) getRegistryURL(registryName string, config *types.Config) string {
	for _, regConfig := range config.Registries {
		if regConfig.Name == registryName {
			url := regConfig.URL
			if strings.HasPrefix(url, "http://") {
				url = strings.TrimPrefix(url, "http://")
			} else if strings.HasPrefix(url, "https://") {
				url = strings.TrimPrefix(url, "https://")
			}
			return url
		}
	}
	return registryName
}

func (m *Manager) getHarborProject(registryName string, config *types.Config) string {
	for _, regConfig := range config.Registries {
		if regConfig.Name == registryName && regConfig.Project != "" {
			return regConfig.Project
		}
	}
	return "library"
}

func (m *Manager) getECRURL(registryName string, config *types.Config) string {
	for _, regConfig := range config.Registries {
		if regConfig.Name == registryName && regConfig.Type == "ecr" {
			if regConfig.AccountID != "" {
				return fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", regConfig.AccountID, regConfig.Region)
			}
		}
	}
	return registryName
}

func (m *Manager) getGHCROrganization(registryName string, config *types.Config) string {
	for _, regConfig := range config.Registries {
		if regConfig.Name == registryName {
			if regConfig.Project != "" {
				return regConfig.Project
			}
			return regConfig.Username
		}
	}
	return "unknown"
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

type ParsedImage struct {
	OriginalImage  string
	Registry       string
	Namespace      string
	Repository     string
	FullRepository string
	Tag            string
	Digest         string
}

func parseImageName(imageName string) *ParsedImage {
	parsed := &ParsedImage{
		OriginalImage: imageName,
		Tag:           "latest",
	}

	workingImage := imageName

	if strings.Contains(workingImage, "@") {
		parts := strings.Split(workingImage, "@")
		workingImage = parts[0]
		parsed.Digest = parts[1]
	}

	if strings.Contains(workingImage, ":") {
		parts := strings.Split(workingImage, ":")
		workingImage = parts[0]
		parsed.Tag = parts[1]
	}

	parts := strings.Split(workingImage, "/")

	switch len(parts) {
	case 1:
		parsed.Registry = "docker.io"
		parsed.Namespace = "library"
		parsed.Repository = parts[0]
		parsed.FullRepository = fmt.Sprintf("library/%s", parts[0])

	case 2:
		if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
			parsed.Registry = parts[0]
			parsed.Namespace = ""
			parsed.Repository = parts[1]
			parsed.FullRepository = parts[1]
		} else {
			parsed.Registry = "docker.io"
			parsed.Namespace = parts[0]
			parsed.Repository = parts[1]
			parsed.FullRepository = fmt.Sprintf("%s/%s", parts[0], parts[1])
		}

	case 3:
		parsed.Registry = parts[0]
		parsed.Namespace = parts[1]
		parsed.Repository = parts[2]
		parsed.FullRepository = fmt.Sprintf("%s/%s", parts[1], parts[2])

	default:
		parsed.Registry = parts[0]
		parsed.Repository = parts[len(parts)-1]
		parsed.Namespace = strings.Join(parts[1:len(parts)-1], "/")
		parsed.FullRepository = strings.Join(parts[1:], "/")
	}

	if parsed.Registry == "index.docker.io" || parsed.Registry == "registry-1.docker.io" {
		parsed.Registry = "docker.io"
	}

	return parsed
}
