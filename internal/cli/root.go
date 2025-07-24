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
	Short: "Migra imagens Docker públicas para registries privados",
	Long: `Privateer é uma ferramenta que escaneia clusters Kubernetes e repositórios GitHub
para encontrar imagens Docker públicas e migrá-las para registries privados.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "arquivo de configuração (padrão: ~/.privateer/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&language, "language", "", "idioma dos logs (pt-BR, en-US, es-ES)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "nível de log (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "executar sem fazer alterações")

	addSubcommands()
}

func addSubcommands() {
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(statusCmd)
}
