package migration

import (
	"testing"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestEngine_generateTargetImageName(t *testing.T) {
	tests := []struct {
		name           string
		image          *types.ImageInfo
		registryType   string
		registryName   string
		config         *types.Config
		expectedResult string
		expectError    bool
	}{
		{
			name: "Docker registry with simple image (becomes library/nginx)",
			image: &types.ImageInfo{
				Image: "nginx:latest",
			},
			registryType: "docker",
			registryName: "docker-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name: "docker-registry",
						Type: "docker",
						URL:  "registry.example.com",
					},
				},
			},
			expectedResult: "registry.example.com/library/nginx:latest",
			expectError:    false,
		},
		{
			name: "Docker registry with namespaced image",
			image: &types.ImageInfo{
				Image: "library/nginx:1.21",
			},
			registryType: "docker",
			registryName: "docker-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name: "docker-registry",
						Type: "docker",
						URL:  "registry.example.com",
					},
				},
			},
			expectedResult: "registry.example.com/library/nginx:1.21",
			expectError:    false,
		},
		{
			name: "Harbor registry with project",
			image: &types.ImageInfo{
				Image: "nginx:latest",
			},
			registryType: "harbor",
			registryName: "harbor-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name:    "harbor-registry",
						Type:    "harbor",
						URL:     "harbor.example.com",
						Project: "my-project",
					},
				},
			},
			expectedResult: "harbor.example.com/my-project/library/nginx:latest",
			expectError:    false,
		},
		{
			name: "Harbor registry without project (default library)",
			image: &types.ImageInfo{
				Image: "nginx:latest",
			},
			registryType: "harbor",
			registryName: "harbor-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name: "harbor-registry",
						Type: "harbor",
						URL:  "harbor.example.com",
					},
				},
			},
			expectedResult: "harbor.example.com/library/library/nginx:latest",
			expectError:    false,
		},
		{
			name: "ECR registry",
			image: &types.ImageInfo{
				Image: "nginx:latest",
			},
			registryType: "ecr",
			registryName: "ecr-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name:      "ecr-registry",
						Type:      "ecr",
						AccountID: "123456789012",
						Region:    "us-east-1",
					},
				},
			},
			expectedResult: "123456789012.dkr.ecr.us-east-1.amazonaws.com/library/nginx:latest",
			expectError:    false,
		},
		{
			name: "GHCR registry with organization",
			image: &types.ImageInfo{
				Image: "nginx:latest",
			},
			registryType: "ghcr",
			registryName: "ghcr-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name:    "ghcr-registry",
						Type:    "ghcr",
						Project: "my-org",
					},
				},
			},
			expectedResult: "ghcr.io/my-org/library/nginx:latest",
			expectError:    false,
		},
		{
			name: "GHCR registry with username fallback",
			image: &types.ImageInfo{
				Image: "nginx:latest",
			},
			registryType: "ghcr",
			registryName: "ghcr-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name:     "ghcr-registry",
						Type:     "ghcr",
						Username: "myuser",
					},
				},
			},
			expectedResult: "ghcr.io/myuser/library/nginx:latest",
			expectError:    false,
		},
		{
			name: "Image with digest",
			image: &types.ImageInfo{
				Image: "nginx:latest@sha256:abc123",
			},
			registryType: "docker",
			registryName: "docker-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name: "docker-registry",
						Type: "docker",
						URL:  "registry.example.com",
					},
				},
			},
			expectedResult: "registry.example.com/library/nginx:latest@sha256:abc123",
			expectError:    false,
		},
		{
			name: "Unknown registry type (default)",
			image: &types.ImageInfo{
				Image: "nginx:latest",
			},
			registryType: "unknown",
			registryName: "unknown-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name: "unknown-registry",
						Type: "unknown",
					},
				},
			},
			expectedResult: "unknown-registry/library/nginx:latest",
			expectError:    false,
		},
		{
			name: "Custom namespace image",
			image: &types.ImageInfo{
				Image: "mycompany/myapp:v1.0",
			},
			registryType: "docker",
			registryName: "docker-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name: "docker-registry",
						Type: "docker",
						URL:  "registry.example.com",
					},
				},
			},
			expectedResult: "registry.example.com/mycompany/myapp:v1.0",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logger.NewTest()
			engine := &TestEngine{
				logger: logger,
				config: tt.config,
			}

			mockReg := &MockRegistry{}
			mockReg.On("GetType").Return(tt.registryType)
			mockReg.On("GetName").Return(tt.registryName)

			result, err := engine.generateTargetImageName(tt.image, mockReg)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			mockReg.AssertExpectations(t)
		})
	}
}

func TestEngine_getRegistryURL(t *testing.T) {
	tests := []struct {
		name         string
		registryName string
		config       *types.Config
		expected     string
	}{
		{
			name:         "Registry found with URL",
			registryName: "test-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name: "test-registry",
						URL:  "registry.example.com",
					},
				},
			},
			expected: "registry.example.com",
		},
		{
			name:         "Registry not found returns registry name",
			registryName: "missing-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name: "test-registry",
						URL:  "registry.example.com",
					},
				},
			},
			expected: "missing-registry",
		},
		{
			name:         "Registry with empty URL returns registry name",
			registryName: "empty-url-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name: "empty-url-registry",
						URL:  "",
					},
				},
			},
			expected: "",
		},
		{
			name:         "Registry with multiple configs returns first match",
			registryName: "test-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name: "other-registry",
						URL:  "other.example.com",
					},
					{
						Name: "test-registry",
						URL:  "registry.example.com",
					},
					{
						Name: "test-registry",
						URL:  "duplicate.example.com",
					},
				},
			},
			expected: "registry.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logger.NewTest()
			engine := &TestEngine{
				logger: logger,
				config: tt.config,
			}

			result := engine.getRegistryURL(tt.registryName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEngine_getHarborProject(t *testing.T) {
	tests := []struct {
		name         string
		registryName string
		config       *types.Config
		expected     string
	}{
		{
			name:         "Registry with project",
			registryName: "harbor-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name:    "harbor-registry",
						Project: "my-project",
					},
				},
			},
			expected: "my-project",
		},
		{
			name:         "Registry without project returns default library",
			registryName: "harbor-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name: "harbor-registry",
					},
				},
			},
			expected: "library",
		},
		{
			name:         "Registry not found returns default library",
			registryName: "missing-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name: "harbor-registry",
					},
				},
			},
			expected: "library",
		},
		{
			name:         "Registry with empty project returns default library",
			registryName: "harbor-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name:    "harbor-registry",
						Project: "",
					},
				},
			},
			expected: "library",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logger.NewTest()
			engine := &TestEngine{
				logger: logger,
				config: tt.config,
			}

			result := engine.getHarborProject(tt.registryName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEngine_getECRURL(t *testing.T) {
	tests := []struct {
		name         string
		registryName string
		config       *types.Config
		expected     string
	}{
		{
			name:         "ECR registry with AccountID and Region",
			registryName: "ecr-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name:      "ecr-registry",
						Type:      "ecr",
						AccountID: "123456789012",
						Region:    "us-east-1",
					},
				},
			},
			expected: "123456789012.dkr.ecr.us-east-1.amazonaws.com",
		},
		{
			name:         "ECR registry without AccountID returns registry name",
			registryName: "ecr-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name:   "ecr-registry",
						Type:   "ecr",
						Region: "us-east-1",
					},
				},
			},
			expected: "ecr-registry",
		},
		{
			name:         "Non-ECR registry returns registry name",
			registryName: "docker-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name: "docker-registry",
						Type: "docker",
					},
				},
			},
			expected: "docker-registry",
		},
		{
			name:         "Registry not found returns registry name",
			registryName: "missing-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name: "ecr-registry",
						Type: "ecr",
					},
				},
			},
			expected: "missing-registry",
		},
		{
			name:         "ECR registry with empty AccountID returns registry name",
			registryName: "ecr-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name:      "ecr-registry",
						Type:      "ecr",
						AccountID: "",
						Region:    "us-east-1",
					},
				},
			},
			expected: "ecr-registry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logger.NewTest()
			engine := &TestEngine{
				logger: logger,
				config: tt.config,
			}

			result := engine.getECRURL(tt.registryName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEngine_getGHCROrganization(t *testing.T) {
	tests := []struct {
		name         string
		registryName string
		config       *types.Config
		expected     string
	}{
		{
			name:         "Registry with project",
			registryName: "ghcr-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name:    "ghcr-registry",
						Project: "my-org",
					},
				},
			},
			expected: "my-org",
		},
		{
			name:         "Registry with username fallback",
			registryName: "ghcr-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name:     "ghcr-registry",
						Username: "myuser",
					},
				},
			},
			expected: "myuser",
		},
		{
			name:         "Registry not found returns unknown",
			registryName: "missing-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name: "ghcr-registry",
					},
				},
			},
			expected: "unknown",
		},
		{
			name:         "Registry without project or username returns unknown",
			registryName: "ghcr-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name:     "ghcr-registry",
						Project:  "",
						Username: "",
					},
				},
			},
			expected: "unknown",
		},
		{
			name:         "Registry with project takes precedence over username",
			registryName: "ghcr-registry",
			config: &types.Config{
				Registries: []types.RegistryConfig{
					{
						Name:     "ghcr-registry",
						Project:  "my-org",
						Username: "myuser",
					},
				},
			},
			expected: "my-org",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logger.NewTest()
			engine := &TestEngine{
				logger: logger,
				config: tt.config,
			}

			result := engine.getGHCROrganization(tt.registryName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEngine_generateTargetImageName_AllRegistryTypes(t *testing.T) {
	logger := logger.NewTest()

	config := &types.Config{
		Registries: []types.RegistryConfig{
			{
				Name: "docker-reg",
				Type: "docker",
				URL:  "docker.example.com",
			},
			{
				Name:    "harbor-reg",
				Type:    "harbor",
				URL:     "harbor.example.com",
				Project: "myproject",
			},
			{
				Name:      "ecr-reg",
				Type:      "ecr",
				AccountID: "123456789012",
				Region:    "us-west-2",
			},
			{
				Name:    "ghcr-reg",
				Type:    "ghcr",
				Project: "myorg",
			},
		},
	}

	engine := &TestEngine{
		logger: logger,
		config: config,
	}

	image := &types.ImageInfo{
		Image: "myapp/service:v1.2.3",
	}

	registryTypes := []struct {
		name     string
		regType  string
		regName  string
		expected string
	}{
		{
			name:     "Docker",
			regType:  "docker",
			regName:  "docker-reg",
			expected: "docker.example.com/myapp/service:v1.2.3",
		},
		{
			name:     "Harbor",
			regType:  "harbor",
			regName:  "harbor-reg",
			expected: "harbor.example.com/myproject/myapp/service:v1.2.3",
		},
		{
			name:     "ECR",
			regType:  "ecr",
			regName:  "ecr-reg",
			expected: "123456789012.dkr.ecr.us-west-2.amazonaws.com/myapp/service:v1.2.3",
		},
		{
			name:     "GHCR",
			regType:  "ghcr",
			regName:  "ghcr-reg",
			expected: "ghcr.io/myorg/myapp/service:v1.2.3",
		},
	}

	for _, tt := range registryTypes {
		t.Run(tt.name, func(t *testing.T) {
			mockReg := &MockRegistry{}
			mockReg.On("GetType").Return(tt.regType)
			mockReg.On("GetName").Return(tt.regName)

			result, err := engine.generateTargetImageName(image, mockReg)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)

			mockReg.AssertExpectations(t)
		})
	}
}

func TestEngine_generateTargetImageName_WithDigest(t *testing.T) {
	logger := logger.NewTest()

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
		logger: logger,
		config: config,
	}

	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "Image with digest",
			image:    "nginx:latest@sha256:abcd1234",
			expected: "registry.example.com/library/nginx:latest@sha256:abcd1234",
		},
		{
			name:     "Namespaced image with digest",
			image:    "mycompany/myapp:v1.0@sha256:efgh5678",
			expected: "registry.example.com/mycompany/myapp:v1.0@sha256:efgh5678",
		},
		{
			name:     "Image without digest",
			image:    "nginx:latest",
			expected: "registry.example.com/library/nginx:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockReg := &MockRegistry{}
			mockReg.On("GetType").Return("docker")
			mockReg.On("GetName").Return("test-registry")

			image := &types.ImageInfo{
				Image: tt.image,
			}

			result, err := engine.generateTargetImageName(image, mockReg)

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)

			mockReg.AssertExpectations(t)
		})
	}
}
