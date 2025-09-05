package kubernetes

import (
	"testing"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

func TestScanner_isPublicImage(t *testing.T) {
	log := logger.NewTest()
	config := &types.Config{
		ImageDetection: types.ImageDetectionConfig{
			IgnoreRegistries:        []string{"ignore.local"},
			CustomPrivateRegistries: []string{"private.company.com"},
			CustomPublicRegistries:  []string{"custom-public.io"},
		},
	}

	scanner := &Scanner{
		logger: log,
		config: config,
	}

	tests := []struct {
		name     string
		image    string
		expected bool
	}{
		{
			name:     "docker hub public image",
			image:    "nginx:latest",
			expected: true,
		},
		{
			name:     "docker hub with explicit registry",
			image:    "docker.io/nginx:latest",
			expected: true,
		},
		{
			name:     "localhost image should be private",
			image:    "localhost:5000/myapp:latest",
			expected: false,
		},
		{
			name:     "127.0.0.1 image should be private",
			image:    "127.0.0.1:5000/myapp:latest",
			expected: false,
		},
		{
			name:     "aws ecr private registry",
			image:    "123456789012.dkr.ecr.us-east-1.amazonaws.com/myapp:latest",
			expected: false,
		},
		{
			name:     "aws ecr public registry",
			image:    "public.ecr.aws/nginx/nginx:latest",
			expected: true,
		},
		{
			name:     "azure private registry",
			image:    "myregistry.azurecr.io/myapp:latest",
			expected: false,
		},
		{
			name:     "azure public registry (mcr)",
			image:    "mcr.microsoft.com/dotnet/core/runtime:3.1",
			expected: true,
		},
		{
			name:     "google private registry (gcr)",
			image:    "gcr.io/my-project/myapp:latest",
			expected: false,
		},
		{
			name:     "google public registry (k8s)",
			image:    "registry.k8s.io/pause:3.5",
			expected: true,
		},
		{
			name:     "github container registry private",
			image:    "ghcr.io/owner/repo:latest",
			expected: false,
		},
		{
			name:     "custom private registry",
			image:    "private.company.com/myapp:latest",
			expected: false,
		},
		{
			name:     "custom public registry",
			image:    "custom-public.io/myapp:latest",
			expected: true,
		},
		{
			name:     "ignored registry",
			image:    "ignore.local/myapp:latest",
			expected: false,
		},
		{
			name:     "custom domain registry",
			image:    "registry.example.com/myapp:latest",
			expected: false,
		},
		{
			name:     "quay.io public registry",
			image:    "quay.io/prometheus/prometheus:latest",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.isPublicImage(tt.image)
			if result != tt.expected {
				t.Errorf("isPublicImage(%q) = %v, expected %v", tt.image, result, tt.expected)
			}
		})
	}
}

func TestScanner_shouldIgnoreRegistry(t *testing.T) {
	log := logger.NewTest()

	tests := []struct {
		name           string
		config         *types.Config
		image          string
		expectedIgnore bool
	}{
		{
			name:           "nil config should not ignore",
			config:         nil,
			image:          "nginx:latest",
			expectedIgnore: false,
		},
		{
			name: "empty ignore list should not ignore",
			config: &types.Config{
				ImageDetection: types.ImageDetectionConfig{
					IgnoreRegistries: []string{},
				},
			},
			image:          "nginx:latest",
			expectedIgnore: false,
		},
		{
			name: "image in ignore list should be ignored",
			config: &types.Config{
				ImageDetection: types.ImageDetectionConfig{
					IgnoreRegistries: []string{"localhost", "registry.local"},
				},
			},
			image:          "localhost:5000/myapp:latest",
			expectedIgnore: true,
		},
		{
			name: "image not in ignore list should not be ignored",
			config: &types.Config{
				ImageDetection: types.ImageDetectionConfig{
					IgnoreRegistries: []string{"localhost", "registry.local"},
				},
			},
			image:          "docker.io/nginx:latest",
			expectedIgnore: false,
		},
		{
			name: "case insensitive matching",
			config: &types.Config{
				ImageDetection: types.ImageDetectionConfig{
					IgnoreRegistries: []string{"Registry.Local"},
				},
			},
			image:          "registry.local/myapp:latest",
			expectedIgnore: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := &Scanner{
				logger: log,
				config: tt.config,
			}

			result := scanner.shouldIgnoreRegistry(tt.image)
			if result != tt.expectedIgnore {
				t.Errorf("shouldIgnoreRegistry(%q) = %v, expected %v", tt.image, result, tt.expectedIgnore)
			}
		})
	}
}

func TestScanner_isCustomPrivateRegistry(t *testing.T) {
	log := logger.NewTest()

	tests := []struct {
		name            string
		config          *types.Config
		image           string
		expectedPrivate bool
	}{
		{
			name:            "nil config should return false",
			config:          nil,
			image:           "private.company.com/myapp:latest",
			expectedPrivate: false,
		},
		{
			name: "empty private list should return false",
			config: &types.Config{
				ImageDetection: types.ImageDetectionConfig{
					CustomPrivateRegistries: []string{},
				},
			},
			image:           "private.company.com/myapp:latest",
			expectedPrivate: false,
		},
		{
			name: "image in private list should return true",
			config: &types.Config{
				ImageDetection: types.ImageDetectionConfig{
					CustomPrivateRegistries: []string{"private.company.com", "internal.registry.io"},
				},
			},
			image:           "private.company.com/myapp:latest",
			expectedPrivate: true,
		},
		{
			name: "image not in private list should return false",
			config: &types.Config{
				ImageDetection: types.ImageDetectionConfig{
					CustomPrivateRegistries: []string{"private.company.com", "internal.registry.io"},
				},
			},
			image:           "docker.io/nginx:latest",
			expectedPrivate: false,
		},
		{
			name: "case insensitive matching",
			config: &types.Config{
				ImageDetection: types.ImageDetectionConfig{
					CustomPrivateRegistries: []string{"Private.Company.Com"},
				},
			},
			image:           "private.company.com/myapp:latest",
			expectedPrivate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := &Scanner{
				logger: log,
				config: tt.config,
			}

			result := scanner.isCustomPrivateRegistry(tt.image)
			if result != tt.expectedPrivate {
				t.Errorf("isCustomPrivateRegistry(%q) = %v, expected %v", tt.image, result, tt.expectedPrivate)
			}
		})
	}
}

func TestScanner_isCustomPublicRegistry(t *testing.T) {
	log := logger.NewTest()

	tests := []struct {
		name           string
		config         *types.Config
		image          string
		expectedPublic bool
	}{
		{
			name:           "nil config should return false",
			config:         nil,
			image:          "custom-public.io/myapp:latest",
			expectedPublic: false,
		},
		{
			name: "empty public list should return false",
			config: &types.Config{
				ImageDetection: types.ImageDetectionConfig{
					CustomPublicRegistries: []string{},
				},
			},
			image:          "custom-public.io/myapp:latest",
			expectedPublic: false,
		},
		{
			name: "image in public list should return true",
			config: &types.Config{
				ImageDetection: types.ImageDetectionConfig{
					CustomPublicRegistries: []string{"custom-public.io", "public.registry.com"},
				},
			},
			image:          "custom-public.io/myapp:latest",
			expectedPublic: true,
		},
		{
			name: "image not in public list should return false",
			config: &types.Config{
				ImageDetection: types.ImageDetectionConfig{
					CustomPublicRegistries: []string{"custom-public.io", "public.registry.com"},
				},
			},
			image:          "docker.io/nginx:latest",
			expectedPublic: false,
		},
		{
			name: "case insensitive matching",
			config: &types.Config{
				ImageDetection: types.ImageDetectionConfig{
					CustomPublicRegistries: []string{"Custom-Public.IO"},
				},
			},
			image:          "custom-public.io/myapp:latest",
			expectedPublic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scanner := &Scanner{
				logger: log,
				config: tt.config,
			}

			result := scanner.isCustomPublicRegistry(tt.image)
			if result != tt.expectedPublic {
				t.Errorf("isCustomPublicRegistry(%q) = %v, expected %v", tt.image, result, tt.expectedPublic)
			}
		})
	}
}

func TestScanner_isPrivateRegistry(t *testing.T) {
	log := logger.NewTest()
	scanner := &Scanner{
		logger: log,
	}

	tests := []struct {
		name            string
		image           string
		expectedPrivate bool
	}{
		{
			name:            "AWS ECR private registry",
			image:           "123456789012.dkr.ecr.us-east-1.amazonaws.com/myapp:latest",
			expectedPrivate: true,
		},
		{
			name:            "AWS ECR public registry should not be private",
			image:           "public.ecr.aws/nginx/nginx:latest",
			expectedPrivate: false,
		},
		{
			name:            "Azure Container Registry private",
			image:           "myregistry.azurecr.io/myapp:latest",
			expectedPrivate: true,
		},
		{
			name:            "Microsoft Container Registry should not be private",
			image:           "mcr.microsoft.com/dotnet/core/runtime:3.1",
			expectedPrivate: false,
		},
		{
			name:            "Google Container Registry private",
			image:           "gcr.io/my-project/myapp:latest",
			expectedPrivate: true,
		},
		{
			name:            "Google Artifact Registry private",
			image:           "us-central1-docker.pkg.dev/my-project/my-repo/myapp:latest",
			expectedPrivate: true,
		},
		{
			name:            "Kubernetes public registry should not be private",
			image:           "registry.k8s.io/pause:3.5",
			expectedPrivate: false,
		},
		{
			name:            "GitHub Container Registry private",
			image:           "ghcr.io/owner/repo:latest",
			expectedPrivate: true,
		},
		{
			name:            "GitHub Container Registry with insufficient parts",
			image:           "ghcr.io/repo",
			expectedPrivate: false,
		},
		{
			name:            "Custom domain registry",
			image:           "registry.company.com/myapp:latest",
			expectedPrivate: true,
		},
		{
			name:            "Docker Hub should not be private",
			image:           "docker.io/nginx:latest",
			expectedPrivate: false,
		},
		{
			name:            "Registry with index.docker.io should not be private",
			image:           "index.docker.io/nginx:latest",
			expectedPrivate: false,
		},
		{
			name:            "Simple image name should not be private",
			image:           "nginx:latest",
			expectedPrivate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scanner.isPrivateRegistry(tt.image)
			if result != tt.expectedPrivate {
				t.Errorf("isPrivateRegistry(%q) = %v, expected %v", tt.image, result, tt.expectedPrivate)
			}
		})
	}
}

func TestScanner_filterPublicImages(t *testing.T) {
	log := logger.NewTest()
	config := &types.Config{
		ImageDetection: types.ImageDetectionConfig{
			CustomPrivateRegistries: []string{"private.company.com"},
		},
	}

	scanner := &Scanner{
		logger: log,
		config: config,
	}

	inputImages := []*types.ImageInfo{
		{
			Image:        "nginx:latest",
			ResourceType: "Deployment",
			ResourceName: "web",
			Namespace:    "default",
		},
		{
			Image:        "private.company.com/myapp:latest",
			ResourceType: "Deployment",
			ResourceName: "app",
			Namespace:    "default",
		},
		{
			Image:        "docker.io/redis:6",
			ResourceType: "StatefulSet",
			ResourceName: "cache",
			Namespace:    "default",
		},
		{
			Image:        "localhost:5000/local:latest",
			ResourceType: "Job",
			ResourceName: "migration",
			Namespace:    "default",
		},
	}

	result := scanner.filterPublicImages(inputImages)

	expectedPublicCount := 2
	if len(result) != expectedPublicCount {
		t.Errorf("filterPublicImages() returned %d images, expected %d", len(result), expectedPublicCount)
	}

	publicImages := make(map[string]bool)
	for _, img := range result {
		publicImages[img.Image] = true
		if !img.IsPublic {
			t.Errorf("Image %s should be marked as public but IsPublic=false", img.Image)
		}
	}

	expectedPublicImages := []string{"nginx:latest", "docker.io/redis:6"}
	for _, expected := range expectedPublicImages {
		if !publicImages[expected] {
			t.Errorf("Expected public image %s not found in results", expected)
		}
	}

	unexpectedPublicImages := []string{"private.company.com/myapp:latest", "localhost:5000/local:latest"}
	for _, unexpected := range unexpectedPublicImages {
		if publicImages[unexpected] {
			t.Errorf("Unexpected public image %s found in results", unexpected)
		}
	}
}
