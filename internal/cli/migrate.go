package cli

import (
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migra imagens públicas para registries privados",
	Long:  "Migra imagens Docker públicas encontradas para registries privados configurados",
}

var migrateClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Migra imagens do cluster",
	Long:  "Migra imagens públicas encontradas no cluster Kubernetes para registries privados",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Info("operation_completed").
			Str("operation", "cluster_migrate").
			Send()
		return nil
	},
}

var migrateGithubCmd = &cobra.Command{
	Use:   "github",
	Short: "Migra imagens dos repositórios GitHub",
	Long:  "Migra imagens públicas encontradas nos repositórios GitHub e cria PRs com as alterações",
	RunE: func(cmd *cobra.Command, args []string) error {
		log.Info("operation_completed").
			Str("operation", "github_migrate").
			Send()
		return nil
	},
}

func init() {
	migrateCmd.AddCommand(migrateClusterCmd)
	migrateCmd.AddCommand(migrateGithubCmd)
}
