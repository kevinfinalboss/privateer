package migration

import (
	"testing"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestEngine_selectTargetRegistries(t *testing.T) {
	tests := []struct {
		name             string
		config           *types.Config
		expectedCount    int
		expectedNames    []string
		expectedPriority []int
	}{
		{
			name: "Multiple registries enabled - multiple mode",
			config: &types.Config{
				Settings: types.SettingsConfig{
					MultipleRegistries: true,
				},
				Registries: []types.RegistryConfig{
					{
						Name:     "registry-high",
						Enabled:  true,
						Priority: 10,
					},
					{
						Name:     "registry-medium",
						Enabled:  true,
						Priority: 5,
					},
					{
						Name:     "registry-low",
						Enabled:  true,
						Priority: 1,
					},
				},
			},
			expectedCount:    3,
			expectedNames:    []string{"registry-high", "registry-medium", "registry-low"},
			expectedPriority: []int{10, 5, 1},
		},
		{
			name: "Multiple registries enabled - single mode",
			config: &types.Config{
				Settings: types.SettingsConfig{
					MultipleRegistries: false,
				},
				Registries: []types.RegistryConfig{
					{
						Name:     "registry-high",
						Enabled:  true,
						Priority: 10,
					},
					{
						Name:     "registry-medium",
						Enabled:  true,
						Priority: 5,
					},
					{
						Name:     "registry-low",
						Enabled:  true,
						Priority: 1,
					},
				},
			},
			expectedCount:    1,
			expectedNames:    []string{"registry-high"},
			expectedPriority: []int{10},
		},
		{
			name: "Some registries disabled",
			config: &types.Config{
				Settings: types.SettingsConfig{
					MultipleRegistries: true,
				},
				Registries: []types.RegistryConfig{
					{
						Name:     "registry-enabled",
						Enabled:  true,
						Priority: 10,
					},
					{
						Name:     "registry-disabled",
						Enabled:  false,
						Priority: 5,
					},
					{
						Name:     "registry-enabled-2",
						Enabled:  true,
						Priority: 3,
					},
				},
			},
			expectedCount:    2,
			expectedNames:    []string{"registry-enabled", "registry-enabled-2"},
			expectedPriority: []int{10, 3},
		},
		{
			name: "No registries enabled",
			config: &types.Config{
				Settings: types.SettingsConfig{
					MultipleRegistries: true,
				},
				Registries: []types.RegistryConfig{
					{
						Name:     "registry-disabled-1",
						Enabled:  false,
						Priority: 10,
					},
					{
						Name:     "registry-disabled-2",
						Enabled:  false,
						Priority: 5,
					},
				},
			},
			expectedCount:    0,
			expectedNames:    []string{},
			expectedPriority: []int{},
		},
		{
			name: "Registries with same priority",
			config: &types.Config{
				Settings: types.SettingsConfig{
					MultipleRegistries: true,
				},
				Registries: []types.RegistryConfig{
					{
						Name:     "registry-1",
						Enabled:  true,
						Priority: 5,
					},
					{
						Name:     "registry-2",
						Enabled:  true,
						Priority: 5,
					},
					{
						Name:     "registry-3",
						Enabled:  true,
						Priority: 10,
					},
				},
			},
			expectedCount: 3,
			expectedNames: []string{"registry-3", "registry-1", "registry-2"},
		},
		{
			name: "Single registry enabled",
			config: &types.Config{
				Settings: types.SettingsConfig{
					MultipleRegistries: false,
				},
				Registries: []types.RegistryConfig{
					{
						Name:     "only-registry",
						Enabled:  true,
						Priority: 5,
					},
				},
			},
			expectedCount:    1,
			expectedNames:    []string{"only-registry"},
			expectedPriority: []int{5},
		},
		{
			name: "Empty registries list",
			config: &types.Config{
				Settings: types.SettingsConfig{
					MultipleRegistries: true,
				},
				Registries: []types.RegistryConfig{},
			},
			expectedCount:    0,
			expectedNames:    []string{},
			expectedPriority: []int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := logger.NewTest()
			engine := &Engine{
				logger: logger,
				config: tt.config,
			}

			result := engine.selectTargetRegistries()

			assert.Equal(t, tt.expectedCount, len(result))

			if len(tt.expectedNames) > 0 {
				for i, expectedName := range tt.expectedNames {
					if i < len(result) {
						assert.Equal(t, expectedName, result[i].Name)
					}
				}
			}

			if len(tt.expectedPriority) > 0 {
				for i, expectedPriority := range tt.expectedPriority {
					if i < len(result) {
						assert.Equal(t, expectedPriority, result[i].Priority)
					}
				}
			}

			for i := 0; i < len(result)-1; i++ {
				assert.GreaterOrEqual(t, result[i].Priority, result[i+1].Priority,
					"Registries should be sorted by priority in descending order")
			}
		})
	}
}

func TestEngine_selectTargetRegistries_PriorityOrdering(t *testing.T) {
	logger := logger.NewTest()

	config := &types.Config{
		Settings: types.SettingsConfig{
			MultipleRegistries: true,
		},
		Registries: []types.RegistryConfig{
			{
				Name:     "registry-medium",
				Enabled:  true,
				Priority: 5,
				Type:     "docker",
			},
			{
				Name:     "registry-highest",
				Enabled:  true,
				Priority: 100,
				Type:     "ecr",
			},
			{
				Name:     "registry-lowest",
				Enabled:  true,
				Priority: 1,
				Type:     "harbor",
			},
			{
				Name:     "registry-high",
				Enabled:  true,
				Priority: 75,
				Type:     "ghcr",
			},
		},
	}

	engine := &Engine{
		logger: logger,
		config: config,
	}

	result := engine.selectTargetRegistries()

	assert.Equal(t, 4, len(result))
	assert.Equal(t, "registry-highest", result[0].Name)
	assert.Equal(t, 100, result[0].Priority)
	assert.Equal(t, "registry-high", result[1].Name)
	assert.Equal(t, 75, result[1].Priority)
	assert.Equal(t, "registry-medium", result[2].Name)
	assert.Equal(t, 5, result[2].Priority)
	assert.Equal(t, "registry-lowest", result[3].Name)
	assert.Equal(t, 1, result[3].Priority)

	for i := 0; i < len(result)-1; i++ {
		assert.GreaterOrEqual(t, result[i].Priority, result[i+1].Priority)
	}
}

func TestEngine_selectTargetRegistries_SingleModeSelectsHighestPriority(t *testing.T) {
	logger := logger.NewTest()

	config := &types.Config{
		Settings: types.SettingsConfig{
			MultipleRegistries: false,
		},
		Registries: []types.RegistryConfig{
			{
				Name:     "registry-medium",
				Enabled:  true,
				Priority: 5,
			},
			{
				Name:     "registry-low",
				Enabled:  true,
				Priority: 1,
			},
			{
				Name:     "registry-highest",
				Enabled:  true,
				Priority: 100,
			},
			{
				Name:     "registry-disabled",
				Enabled:  false,
				Priority: 200,
			},
		},
	}

	engine := &Engine{
		logger: logger,
		config: config,
	}

	result := engine.selectTargetRegistries()

	assert.Equal(t, 1, len(result))
	assert.Equal(t, "registry-highest", result[0].Name)
	assert.Equal(t, 100, result[0].Priority)
}

func TestEngine_selectTargetRegistries_EmptyConfigHandling(t *testing.T) {
	logger := logger.NewTest()

	config := &types.Config{
		Settings: types.SettingsConfig{
			MultipleRegistries: true,
		},
		Registries: nil,
	}

	engine := &Engine{
		logger: logger,
		config: config,
	}

	result := engine.selectTargetRegistries()

	assert.Equal(t, 0, len(result))
	assert.Empty(t, result)
}

func TestEngine_selectTargetRegistries_AllDisabledRegistries(t *testing.T) {
	logger := logger.NewTest()

	config := &types.Config{
		Settings: types.SettingsConfig{
			MultipleRegistries: true,
		},
		Registries: []types.RegistryConfig{
			{
				Name:     "registry-1",
				Enabled:  false,
				Priority: 10,
			},
			{
				Name:     "registry-2",
				Enabled:  false,
				Priority: 5,
			},
			{
				Name:     "registry-3",
				Enabled:  false,
				Priority: 1,
			},
		},
	}

	engine := &Engine{
		logger: logger,
		config: config,
	}

	result := engine.selectTargetRegistries()

	assert.Equal(t, 0, len(result))
	assert.Empty(t, result)
}
