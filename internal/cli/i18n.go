package cli

import (
	"github.com/kevinfinalboss/privateer/internal/config"
	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

var i18n *logger.Logger

func initI18n() {
	if cfgFile != "" {
		if tempCfg, err := config.Load(cfgFile); err == nil && tempCfg != nil {
			i18n = logger.NewWithConfig(tempCfg)
			return
		}
	}

	defaultCfg := &types.Config{
		Settings: types.SettingsConfig{
			Language: getLanguageFromFlags(),
			LogLevel: "info",
		},
	}

	i18n = logger.NewWithConfig(defaultCfg)
}

func getLanguageFromFlags() string {
	if language != "" {
		return language
	}
	return "pt-BR"
}

func getMessage(key string) string {
	if i18n == nil {
		initI18n()
	}
	return i18n.GetMessage(key)
}
