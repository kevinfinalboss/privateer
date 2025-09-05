package migration

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/internal/registry"
	"github.com/kevinfinalboss/privateer/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type RegistryManagerInterface interface {
	GetRegistry(name string) (registry.Registry, error)
	ValidateImageDuplication(ctx context.Context, targetImage string) error
	RemoveLocalImage(ctx context.Context, imageName string) error
	AddRegistry(config *types.RegistryConfig) error
	HealthCheck(ctx context.Context) error
	CheckImageExists(ctx context.Context, imageName string) (map[string]bool, error)
	ListRegistries() []string
	GetEnabledRegistries() []registry.Registry
	ValidateImagesBatch(ctx context.Context, images []*types.ImageInfo, config *types.Config) (map[string]string, error)
	FindImageInRegistries(ctx context.Context, publicImage *types.ImageInfo, config *types.Config) (string, string, error)
	GetRegistryCount() int
}

type MockRegistryManager struct {
	mock.Mock
}

func (m *MockRegistryManager) GetRegistry(name string) (registry.Registry, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(registry.Registry), args.Error(1)
}

func (m *MockRegistryManager) ValidateImageDuplication(ctx context.Context, targetImage string) error {
	args := m.Called(ctx, targetImage)
	return args.Error(0)
}

func (m *MockRegistryManager) RemoveLocalImage(ctx context.Context, imageName string) error {
	args := m.Called(ctx, imageName)
	return args.Error(0)
}

func (m *MockRegistryManager) AddRegistry(config *types.RegistryConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockRegistryManager) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRegistryManager) CheckImageExists(ctx context.Context, imageName string) (map[string]bool, error) {
	args := m.Called(ctx, imageName)
	return args.Get(0).(map[string]bool), args.Error(1)
}

func (m *MockRegistryManager) ListRegistries() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockRegistryManager) GetEnabledRegistries() []registry.Registry {
	args := m.Called()
	return args.Get(0).([]registry.Registry)
}

func (m *MockRegistryManager) ValidateImagesBatch(ctx context.Context, images []*types.ImageInfo, config *types.Config) (map[string]string, error) {
	args := m.Called(ctx, images, config)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockRegistryManager) FindImageInRegistries(ctx context.Context, publicImage *types.ImageInfo, config *types.Config) (string, string, error) {
	args := m.Called(ctx, publicImage, config)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockRegistryManager) GetRegistryCount() int {
	args := m.Called()
	return args.Int(0)
}

type MockRegistry struct {
	mock.Mock
}

func (m *MockRegistry) Login(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRegistry) Pull(ctx context.Context, imageName string) error {
	args := m.Called(ctx, imageName)
	return args.Error(0)
}

func (m *MockRegistry) Push(ctx context.Context, image *types.ImageInfo, targetTag string) error {
	args := m.Called(ctx, image, targetTag)
	return args.Error(0)
}

func (m *MockRegistry) Copy(ctx context.Context, sourceImage, targetImage string) error {
	args := m.Called(ctx, sourceImage, targetImage)
	return args.Error(0)
}

func (m *MockRegistry) IsHealthy(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockRegistry) HasImage(ctx context.Context, imageName string) (bool, error) {
	args := m.Called(ctx, imageName)
	return args.Bool(0), args.Error(1)
}

func (m *MockRegistry) GetType() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockRegistry) GetName() string {
	args := m.Called()
	return args.String(0)
}

type TestEngine struct {
	logger          *logger.Logger
	registryManager RegistryManagerInterface
	config          *types.Config
}

func (e *TestEngine) migrateImageToRegistry(ctx context.Context, image *types.ImageInfo, registryName string) *types.MigrationResult {
	reg, err := e.registryManager.GetRegistry(registryName)
	if err != nil {
		return &types.MigrationResult{
			Image:    image,
			Registry: registryName,
			Success:  false,
			Error:    err,
		}
	}

	targetImage, err := e.generateTargetImageName(image, reg)
	if err != nil {
		return &types.MigrationResult{
			Image:    image,
			Registry: registryName,
			Success:  false,
			Error:    err,
		}
	}

	if err := e.validateImageDuplication(ctx, targetImage, registryName, image); err != nil {
		return &types.MigrationResult{
			Image:       image,
			TargetImage: targetImage,
			Registry:    registryName,
			Success:     false,
			Skipped:     true,
			Reason:      "Imagem já existe no registry",
			Error:       err,
		}
	}

	if err := e.authenticateRegistry(ctx, reg, registryName); err != nil {
		return &types.MigrationResult{
			Image:    image,
			Registry: registryName,
			Success:  false,
			Error:    err,
		}
	}

	if err := e.copyImage(ctx, reg, image, targetImage, registryName); err != nil {
		return &types.MigrationResult{
			Image:       image,
			TargetImage: targetImage,
			Registry:    registryName,
			Success:     false,
			Error:       err,
		}
	}

	e.cleanupLocalImage(ctx, image.Image)

	return &types.MigrationResult{
		Image:       image,
		TargetImage: targetImage,
		Registry:    registryName,
		Success:     true,
	}
}

func (e *TestEngine) generateTargetImageName(image *types.ImageInfo, reg registry.Registry) (string, error) {
	parsed := types.ParseImageName(image.Image)
	targetRepository := parsed.FullRepository
	targetTag := parsed.Tag

	if parsed.Digest != "" {
		targetTag = fmt.Sprintf("%s@%s", targetTag, parsed.Digest)
	}

	switch reg.GetType() {
	case "docker":
		registryURL := e.getRegistryURL(reg.GetName())
		return fmt.Sprintf("%s/%s:%s", registryURL, targetRepository, targetTag), nil
	case "harbor":
		registryURL := e.getRegistryURL(reg.GetName())
		project := e.getHarborProject(reg.GetName())
		return fmt.Sprintf("%s/%s/%s:%s", registryURL, project, targetRepository, targetTag), nil
	case "ecr":
		ecrURL := e.getECRURL(reg.GetName())
		return fmt.Sprintf("%s/%s:%s", ecrURL, targetRepository, targetTag), nil
	case "ghcr":
		organization := e.getGHCROrganization(reg.GetName())
		return fmt.Sprintf("ghcr.io/%s/%s:%s", organization, targetRepository, targetTag), nil
	default:
		return fmt.Sprintf("%s/%s:%s", reg.GetName(), targetRepository, targetTag), nil
	}
}

func (e *TestEngine) getRegistryURL(registryName string) string {
	for _, regConfig := range e.config.Registries {
		if regConfig.Name == registryName {
			return regConfig.URL
		}
	}
	return registryName
}

func (e *TestEngine) getHarborProject(registryName string) string {
	for _, regConfig := range e.config.Registries {
		if regConfig.Name == registryName && regConfig.Project != "" {
			return regConfig.Project
		}
	}
	return "library"
}

func (e *TestEngine) getECRURL(registryName string) string {
	for _, regConfig := range e.config.Registries {
		if regConfig.Name == registryName && regConfig.Type == "ecr" {
			if regConfig.AccountID != "" {
				return fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", regConfig.AccountID, regConfig.Region)
			}
		}
	}
	return registryName
}

func (e *TestEngine) getGHCROrganization(registryName string) string {
	for _, regConfig := range e.config.Registries {
		if regConfig.Name == registryName {
			if regConfig.Project != "" {
				return regConfig.Project
			}
			if regConfig.Username != "" {
				return regConfig.Username
			}
		}
	}
	return "unknown"
}

func (e *TestEngine) validateImageDuplication(ctx context.Context, targetImage, registryName string, image *types.ImageInfo) error {
	return e.registryManager.ValidateImageDuplication(ctx, targetImage)
}

func (e *TestEngine) authenticateRegistry(ctx context.Context, reg registry.Registry, registryName string) error {
	return reg.Login(ctx)
}

func (e *TestEngine) copyImage(ctx context.Context, reg registry.Registry, image *types.ImageInfo, targetImage, registryName string) error {
	return reg.Copy(ctx, image.Image, targetImage)
}

func (e *TestEngine) cleanupLocalImage(ctx context.Context, imageName string) error {
	return e.registryManager.RemoveLocalImage(ctx, imageName)
}

func TestEngine_migrateImageToRegistry_Success(t *testing.T) {
	logger := logger.NewTest()
	mockRegistryManager := &MockRegistryManager{}
	mockRegistry := &MockRegistry{}

	config := &types.Config{
		Registries: []types.RegistryConfig{
			{
				Name: "test-registry",
				Type: "docker",
				URL:  "registry.example.com",
			},
		},
	}

	engine := &TestEngine{
		logger:          logger,
		registryManager: mockRegistryManager,
		config:          config,
	}

	image := &types.ImageInfo{
		Image:     "library/nginx:latest",
		Namespace: "default",
		Container: "nginx",
	}

	mockRegistry.On("GetType").Return("docker")
	mockRegistry.On("GetName").Return("test-registry")
	mockRegistryManager.On("GetRegistry", "test-registry").Return(mockRegistry, nil)
	mockRegistryManager.On("ValidateImageDuplication", mock.Anything, "registry.example.com/library/nginx:latest").Return(nil)
	mockRegistry.On("Login", mock.Anything).Return(nil)
	mockRegistry.On("Copy", mock.Anything, "library/nginx:latest", "registry.example.com/library/nginx:latest").Return(nil)
	mockRegistryManager.On("RemoveLocalImage", mock.Anything, "library/nginx:latest").Return(nil)

	result := engine.migrateImageToRegistry(context.Background(), image, "test-registry")

	assert.True(t, result.Success)
	assert.False(t, result.Skipped)
	assert.Nil(t, result.Error)
	assert.Equal(t, image, result.Image)
	assert.Equal(t, "test-registry", result.Registry)
	assert.Equal(t, "registry.example.com/library/nginx:latest", result.TargetImage)

	mockRegistryManager.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)
}

func TestEngine_migrateImageToRegistry_RegistryNotFound(t *testing.T) {
	logger := logger.NewTest()
	mockRegistryManager := &MockRegistryManager{}

	engine := &TestEngine{
		logger:          logger,
		registryManager: mockRegistryManager,
	}

	image := &types.ImageInfo{
		Image:     "nginx:latest",
		Namespace: "default",
	}

	mockRegistryManager.On("GetRegistry", "missing-registry").Return(nil, errors.New("registry não encontrado"))

	result := engine.migrateImageToRegistry(context.Background(), image, "missing-registry")

	assert.False(t, result.Success)
	assert.False(t, result.Skipped)
	assert.NotNil(t, result.Error)
	assert.Equal(t, image, result.Image)
	assert.Equal(t, "missing-registry", result.Registry)

	mockRegistryManager.AssertExpectations(t)
}

func TestEngine_migrateImageToRegistry_ImageDuplicationDetected(t *testing.T) {
	logger := logger.NewTest()
	mockRegistryManager := &MockRegistryManager{}
	mockRegistry := &MockRegistry{}

	config := &types.Config{
		Registries: []types.RegistryConfig{
			{
				Name: "test-registry",
				Type: "docker",
				URL:  "registry.example.com",
			},
		},
	}

	engine := &TestEngine{
		logger:          logger,
		registryManager: mockRegistryManager,
		config:          config,
	}

	image := &types.ImageInfo{
		Image:     "library/nginx:latest",
		Namespace: "default",
	}

	mockRegistry.On("GetType").Return("docker")
	mockRegistry.On("GetName").Return("test-registry")
	mockRegistryManager.On("GetRegistry", "test-registry").Return(mockRegistry, nil)
	mockRegistryManager.On("ValidateImageDuplication", mock.Anything, "registry.example.com/library/nginx:latest").Return(errors.New("imagem já existe"))

	result := engine.migrateImageToRegistry(context.Background(), image, "test-registry")

	assert.False(t, result.Success)
	assert.True(t, result.Skipped)
	assert.NotNil(t, result.Error)
	assert.Equal(t, "Imagem já existe no registry", result.Reason)
	assert.Equal(t, image, result.Image)
	assert.Equal(t, "test-registry", result.Registry)
	assert.Equal(t, "registry.example.com/library/nginx:latest", result.TargetImage)

	mockRegistryManager.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)
}

func TestEngine_migrateImageToRegistry_LoginFailure(t *testing.T) {
	logger := logger.NewTest()
	mockRegistryManager := &MockRegistryManager{}
	mockRegistry := &MockRegistry{}

	config := &types.Config{
		Registries: []types.RegistryConfig{
			{
				Name: "test-registry",
				Type: "docker",
				URL:  "registry.example.com",
			},
		},
	}

	engine := &TestEngine{
		logger:          logger,
		registryManager: mockRegistryManager,
		config:          config,
	}

	image := &types.ImageInfo{
		Image:     "library/nginx:latest",
		Namespace: "default",
	}

	mockRegistry.On("GetType").Return("docker")
	mockRegistry.On("GetName").Return("test-registry")
	mockRegistryManager.On("GetRegistry", "test-registry").Return(mockRegistry, nil)
	mockRegistryManager.On("ValidateImageDuplication", mock.Anything, "registry.example.com/library/nginx:latest").Return(nil)
	mockRegistry.On("Login", mock.Anything).Return(errors.New("authentication failed"))

	result := engine.migrateImageToRegistry(context.Background(), image, "test-registry")

	assert.False(t, result.Success)
	assert.False(t, result.Skipped)
	assert.NotNil(t, result.Error)
	assert.Equal(t, image, result.Image)
	assert.Equal(t, "test-registry", result.Registry)

	mockRegistryManager.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)
}

func TestEngine_migrateImageToRegistry_CopyFailure(t *testing.T) {
	logger := logger.NewTest()
	mockRegistryManager := &MockRegistryManager{}
	mockRegistry := &MockRegistry{}

	config := &types.Config{
		Registries: []types.RegistryConfig{
			{
				Name: "test-registry",
				Type: "docker",
				URL:  "registry.example.com",
			},
		},
	}

	engine := &TestEngine{
		logger:          logger,
		registryManager: mockRegistryManager,
		config:          config,
	}

	image := &types.ImageInfo{
		Image:     "library/nginx:latest",
		Namespace: "default",
	}

	mockRegistry.On("GetType").Return("docker")
	mockRegistry.On("GetName").Return("test-registry")
	mockRegistryManager.On("GetRegistry", "test-registry").Return(mockRegistry, nil)
	mockRegistryManager.On("ValidateImageDuplication", mock.Anything, "registry.example.com/library/nginx:latest").Return(nil)
	mockRegistry.On("Login", mock.Anything).Return(nil)
	mockRegistry.On("Copy", mock.Anything, "library/nginx:latest", "registry.example.com/library/nginx:latest").Return(errors.New("copy failed"))

	result := engine.migrateImageToRegistry(context.Background(), image, "test-registry")

	assert.False(t, result.Success)
	assert.False(t, result.Skipped)
	assert.NotNil(t, result.Error)
	assert.Equal(t, image, result.Image)
	assert.Equal(t, "test-registry", result.Registry)
	assert.Equal(t, "registry.example.com/library/nginx:latest", result.TargetImage)

	mockRegistryManager.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)
}

func TestEngine_migrateImageToRegistry_WithHarborRegistry(t *testing.T) {
	logger := logger.NewTest()
	mockRegistryManager := &MockRegistryManager{}
	mockRegistry := &MockRegistry{}

	config := &types.Config{
		Registries: []types.RegistryConfig{
			{
				Name:    "harbor-registry",
				Type:    "harbor",
				URL:     "harbor.example.com",
				Project: "myproject",
			},
		},
	}

	engine := &TestEngine{
		logger:          logger,
		registryManager: mockRegistryManager,
		config:          config,
	}

	image := &types.ImageInfo{
		Image:     "library/nginx:latest",
		Namespace: "default",
	}

	mockRegistry.On("GetType").Return("harbor")
	mockRegistry.On("GetName").Return("harbor-registry")
	mockRegistryManager.On("GetRegistry", "harbor-registry").Return(mockRegistry, nil)
	mockRegistryManager.On("ValidateImageDuplication", mock.Anything, "harbor.example.com/myproject/library/nginx:latest").Return(nil)
	mockRegistry.On("Login", mock.Anything).Return(nil)
	mockRegistry.On("Copy", mock.Anything, "library/nginx:latest", "harbor.example.com/myproject/library/nginx:latest").Return(nil)
	mockRegistryManager.On("RemoveLocalImage", mock.Anything, "library/nginx:latest").Return(nil)

	result := engine.migrateImageToRegistry(context.Background(), image, "harbor-registry")

	assert.True(t, result.Success)
	assert.False(t, result.Skipped)
	assert.Nil(t, result.Error)
	assert.Equal(t, "harbor.example.com/myproject/library/nginx:latest", result.TargetImage)

	mockRegistryManager.AssertExpectations(t)
	mockRegistry.AssertExpectations(t)
}
