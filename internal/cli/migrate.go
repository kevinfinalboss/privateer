package cli

import (
	"context"
	"fmt"

	"github.com/kevinfinalboss/privateer/internal/kubernetes"
	"github.com/kevinfinalboss/privateer/internal/migration"
	"github.com/kevinfinalboss/privateer/internal/registry"
	"github.com/kevinfinalboss/privateer/pkg/types"
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
		return migrateCluster()
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

func migrateCluster() error {
	ctx := context.Background()

	if len(cfg.Registries) == 0 {
		log.Error("no_registries_configured").Send()
		return fmt.Errorf("nenhum registry configurado. Execute 'privateer init' para configurar")
	}

	registryManager := registry.NewManager(log)
	for _, regConfig := range cfg.Registries {
		if err := registryManager.AddRegistry(&regConfig); err != nil {
			log.Error("registry_add_failed").
				Str("name", regConfig.Name).
				Err(err).
				Send()
			return err
		}
	}

	if err := registryManager.HealthCheck(ctx); err != nil {
		log.Error("registry_health_check_failed").
			Err(err).
			Send()
		return err
	}

	client, err := kubernetes.NewClient(cfg, log)
	if err != nil {
		return err
	}

	namespaces, err := client.GetNamespaces()
	if err != nil {
		log.Error("operation_failed").Err(err).Send()
		return err
	}

	log.Info("migration_cluster_started").
		Int("namespace_count", len(namespaces)).
		Strs("namespaces", namespaces).
		Bool("dry_run", cfg.Settings.DryRun).
		Str("message", func() string {
			if cfg.Settings.DryRun {
				return "SIMULAÇÃO - Nenhuma alteração será feita"
			}
			return "MIGRAÇÃO REAL - Imagens serão copiadas para o registry privado"
		}()).
		Send()

	scanner := kubernetes.NewScanner(client, log, cfg)
	var allPublicImages []*types.ImageInfo

	for _, namespace := range namespaces {
		publicImages, err := scanner.ScanNamespace(namespace)
		if err != nil {
			log.Error("operation_failed").
				Str("namespace", namespace).
				Err(err).
				Send()
			continue
		}
		allPublicImages = append(allPublicImages, publicImages...)
	}

	if len(allPublicImages) == 0 {
		log.Info("no_public_images_found").Send()
		return nil
	}

	log.Info("public_images_found").
		Int("total", len(allPublicImages)).
		Send()

	migrationEngine := migration.NewEngine(registryManager, log, cfg)
	summary, err := migrationEngine.MigrateImages(ctx, allPublicImages)
	if err != nil {
		log.Error("migration_failed").
			Err(err).
			Send()
		return err
	}

	log.Info("migration_summary").
		Int("total", summary.TotalImages).
		Int("success", summary.SuccessCount).
		Int("failures", summary.FailureCount).
		Str("status", func() string {
			if cfg.Settings.DryRun {
				return "SIMULAÇÃO CONCLUÍDA"
			}
			return "MIGRAÇÃO CONCLUÍDA"
		}()).
		Send()

	if summary.FailureCount > 0 {
		log.Warn("migration_had_failures").
			Int("failures", summary.FailureCount).
			Send()

		for _, result := range summary.Results {
			if !result.Success {
				log.Error("migration_failure_detail").
					Str("image", result.Image.Image).
					Str("registry", result.Registry).
					Err(result.Error).
					Send()
			}
		}
	}

	log.Info("operation_completed").
		Str("operation", "cluster_migrate").
		Send()

	return nil
}
