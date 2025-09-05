package migration

import (
	"sort"

	"github.com/kevinfinalboss/privateer/pkg/types"
)

func (e *Engine) selectTargetRegistries() []types.RegistryConfig {
	var enabledRegistries []types.RegistryConfig

	e.logger.Debug("selecting_target_registries").
		Int("total_registries", len(e.config.Registries)).
		Send()

	for i, regConfig := range e.config.Registries {
		e.logger.Debug("checking_registry_config").
			Int("index", i).
			Str("name", regConfig.Name).
			Str("type", regConfig.Type).
			Bool("enabled", regConfig.Enabled).
			Int("priority", regConfig.Priority).
			Send()

		if regConfig.Enabled {
			enabledRegistries = append(enabledRegistries, regConfig)
			e.logger.Debug("registry_added_to_enabled_list").
				Str("name", regConfig.Name).
				Int("priority", regConfig.Priority).
				Send()
		}
	}

	e.logger.Debug("enabled_registries_summary").
		Int("enabled_count", len(enabledRegistries)).
		Send()

	if len(enabledRegistries) == 0 {
		e.logger.Error("no_enabled_registries_found").Send()
		return []types.RegistryConfig{}
	}

	sort.Slice(enabledRegistries, func(i, j int) bool {
		return enabledRegistries[i].Priority > enabledRegistries[j].Priority
	})

	if e.config.Settings.MultipleRegistries {
		e.logger.Info("multiple_registries_mode").
			Int("count", len(enabledRegistries)).
			Send()
		return enabledRegistries
	}

	e.logger.Info("single_registry_mode").
		Str("selected", enabledRegistries[0].Name).
		Int("priority", enabledRegistries[0].Priority).
		Send()

	return []types.RegistryConfig{enabledRegistries[0]}
}
