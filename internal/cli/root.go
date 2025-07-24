package cli

import (
	"fmt"

	"github.com/kevinfinalboss/privateer/internal/config"
	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/spf13/cobra"
)

var (
	cfgFile  string
	language string
	logLevel string
	dryRun   bool
	log      *logger.Logger
	cfg      *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "privateer",
	Short: getMessage("root_short"),
	Long:  getMessage("root_long"),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// O comando init tem seu próprio PreRunE
		if cmd.Name() == "init" {
			return nil
		}

		var err error

		cfg, err = config.Load(cfgFile)
		if err != nil && cfgFile != "" {
			return fmt.Errorf("erro ao carregar configuração: %w", err)
		}

		if cfg == nil {
			cfg = &config.Config{}
		}

		if language != "" {
			cfg.Settings.Language = language
		}
		if logLevel != "" {
			cfg.Settings.LogLevel = logLevel
		}
		if cmd.Flags().Changed("dry-run") {
			cfg.Settings.DryRun = dryRun
		}

		log = logger.NewWithConfig(cfg)

		if cfgFile == "" {
			log.Warn("config_not_found").Send()
		} else {
			log.Info("config_loaded").Str("file", cfgFile).Send()
		}

		log.Info("app_started").
			Str("version", "v0.1.0").
			Str("language", cfg.Settings.Language).
			Bool("dry_run", cfg.Settings.DryRun).
			Send()

		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	initI18n()

	rootCmd.Short = getMessage("root_short")
	rootCmd.Long = getMessage("root_long")

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", getMessage("flag_config"))
	rootCmd.PersistentFlags().StringVar(&language, "language", "", getMessage("flag_language"))
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", getMessage("flag_log_level"))
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, getMessage("flag_dry_run"))

	addSubcommands()
}

func addSubcommands() {
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(statusCmd)
}
