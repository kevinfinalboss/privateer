package cli

import (
	"context"
	"fmt"

	"github.com/kevinfinalboss/privateer/internal/github"
	"github.com/kevinfinalboss/privateer/internal/gitops"
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
		return migrateGithub()
	},
}

var migrateAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Migra imagens do cluster e repositórios GitHub",
	Long:  "Executa migração completa: cluster → registries privados → atualização de repositórios GitHub",
	RunE: func(cmd *cobra.Command, args []string) error {
		return migrateAll()
	},
}

func init() {
	migrateCmd.AddCommand(migrateClusterCmd)
	migrateCmd.AddCommand(migrateGithubCmd)
	migrateCmd.AddCommand(migrateAllCmd)
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

func migrateGithub() error {
	ctx := context.Background()

	if !cfg.GitHub.Enabled {
		log.Error("github_not_enabled").
			Str("message", "GitHub não está habilitado na configuração").
			Send()
		return fmt.Errorf("GitHub não está habilitado. Configure github.enabled: true")
	}

	if !cfg.GitOps.Enabled {
		log.Error("gitops_not_enabled").
			Str("message", "GitOps não está habilitado na configuração").
			Send()
		return fmt.Errorf("GitOps não está habilitado. Configure gitops.enabled: true")
	}

	if cfg.GitHub.Token == "" {
		log.Error("github_token_missing").
			Str("message", "Token GitHub não configurado").
			Send()
		return fmt.Errorf("token GitHub não configurado. Configure github.token")
	}

	enabledRepos := 0
	for _, repo := range cfg.GitHub.Repositories {
		if repo.Enabled {
			enabledRepos++
		}
	}

	if enabledRepos == 0 {
		log.Error("no_github_repositories").
			Str("message", "Nenhum repositório GitHub habilitado").
			Send()
		return fmt.Errorf("nenhum repositório GitHub habilitado encontrado")
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

	log.Info("github_migration_started").
		Int("enabled_repositories", enabledRepos).
		Bool("dry_run", cfg.Settings.DryRun).
		Bool("auto_pr", cfg.GitOps.AutoPR).
		Send()

	publicImages, err := scanClusterImages(client)
	if err != nil {
		return fmt.Errorf("falha ao escanear imagens do cluster: %w", err)
	}

	if len(publicImages) == 0 {
		log.Info("no_public_images_for_github").
			Str("message", "Nenhuma imagem pública encontrada no cluster").
			Send()
		return nil
	}

	githubClient := github.NewClient(&cfg.GitHub, log)
	gitopsEngine := gitops.NewEngine(githubClient, registryManager, log, cfg)

	summary, err := gitopsEngine.MigrateRepositories(ctx, publicImages)
	if err != nil {
		log.Error("github_migration_failed").
			Err(err).
			Send()
		return err
	}

	log.Info("github_migration_summary").
		Int("repositories_processed", summary.ProcessedRepositories).
		Int("successful_prs", summary.SuccessfulPRs).
		Int("failed_operations", summary.FailedOperations).
		Int("files_changed", summary.TotalFilesChanged).
		Int("images_replaced", summary.TotalImagesReplaced).
		Str("processing_time", summary.ProcessingTime).
		Send()

	if len(summary.Results) > 0 {
		for _, result := range summary.Results {
			if result.Success && result.PullRequest != nil {
				log.Info("pull_request_created").
					Str("repository", result.Repository).
					Int("pr_number", result.PullRequest.Number).
					Str("url", result.PullRequest.URL).
					Send()
			} else if !result.Success {
				log.Error("repository_migration_failed").
					Str("repository", result.Repository).
					Err(result.Error).
					Send()
			}
		}
	}

	if summary.FailedOperations > 0 {
		log.Warn("github_migration_had_failures").
			Int("failures", summary.FailedOperations).
			Send()
	}

	log.Info("operation_completed").
		Str("operation", "github_migrate").
		Send()

	return nil
}

func migrateAll() error {
	log.Info("full_migration_started").
		Str("message", "Executando migração completa: cluster → registries → GitHub").
		Send()

	log.Info("phase_1_cluster_migration").Send()
	if err := migrateCluster(); err != nil {
		log.Error("phase_1_failed").
			Err(err).
			Send()
		return fmt.Errorf("falha na migração do cluster: %w", err)
	}

	if cfg.GitHub.Enabled && cfg.GitOps.Enabled {
		log.Info("phase_2_github_migration").Send()
		if err := migrateGithub(); err != nil {
			log.Error("phase_2_failed").
				Err(err).
				Send()
			return fmt.Errorf("falha na migração do GitHub: %w", err)
		}
	} else {
		log.Info("phase_2_skipped").
			Str("message", "GitHub/GitOps não habilitado - pulando atualização de repositórios").
			Send()
	}

	log.Info("full_migration_completed").
		Str("message", "Migração completa finalizada com sucesso").
		Send()

	return nil
}

func scanClusterImages(client *kubernetes.Client) ([]*types.ImageInfo, error) {
	namespaces, err := client.GetNamespaces()
	if err != nil {
		return nil, err
	}

	scanner := kubernetes.NewScanner(client, log, cfg)
	var allPublicImages []*types.ImageInfo

	for _, namespace := range namespaces {
		publicImages, err := scanner.ScanNamespace(namespace)
		if err != nil {
			log.Error("namespace_scan_failed").
				Str("namespace", namespace).
				Err(err).
				Send()
			continue
		}
		allPublicImages = append(allPublicImages, publicImages...)
	}

	return allPublicImages, nil
}
