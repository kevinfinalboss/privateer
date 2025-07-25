package cli

import (
	"github.com/kevinfinalboss/privateer/internal/kubernetes"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: getMessage("scan_short"),
	Long:  getMessage("scan_long"),
}

var scanClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: getMessage("scan_cluster_short"),
	Long:  getMessage("scan_cluster_long"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return scanCluster()
	},
}

var scanGithubCmd = &cobra.Command{
	Use:   "github",
	Short: getMessage("scan_github_short"),
	Long:  getMessage("scan_github_long"),
	RunE: func(cmd *cobra.Command, args []string) error {
		return scanGithub()
	},
}

func init() {
	scanCmd.Short = getMessage("scan_short")
	scanCmd.Long = getMessage("scan_long")
	scanClusterCmd.Short = getMessage("scan_cluster_short")
	scanClusterCmd.Long = getMessage("scan_cluster_long")
	scanGithubCmd.Short = getMessage("scan_github_short")
	scanGithubCmd.Long = getMessage("scan_github_long")

	scanCmd.AddCommand(scanClusterCmd)
	scanCmd.AddCommand(scanGithubCmd)
}

func scanCluster() error {
	client, err := kubernetes.NewClient(cfg, log)
	if err != nil {
		return err
	}

	namespaces, err := client.GetNamespaces()
	if err != nil {
		log.Error("operation_failed").Err(err).Send()
		return err
	}

	log.Info("scanning_cluster").
		Int("namespace_count", len(namespaces)).
		Strs("namespaces", namespaces).
		Send()

	scanner := kubernetes.NewScanner(client, log, cfg)
	totalPublicImages := 0

	for _, namespace := range namespaces {
		publicImages, err := scanner.ScanNamespace(namespace)
		if err != nil {
			log.Error("operation_failed").
				Str("namespace", namespace).
				Err(err).
				Send()
			continue
		}

		if len(publicImages) > 0 {
			totalPublicImages += len(publicImages)

			for _, image := range publicImages {
				log.Info("public_image_found").
					Str("image", image.Image).
					Str("resource_type", image.ResourceType).
					Str("resource_name", image.ResourceName).
					Str("namespace", image.Namespace).
					Str("container", image.Container).
					Bool("is_init_container", image.IsInitContainer).
					Send()
			}
		}
	}

	log.Info("operation_completed").
		Str("operation", "cluster_scan").
		Int("total_public_images", totalPublicImages).
		Send()

	return nil
}

func scanGithub() error {
	if !cfg.GitHub.Enabled {
		log.Warn("github_not_enabled").
			Str("message", "GitHub scanning não está habilitado na configuração").
			Send()
		return nil
	}

	if cfg.GitHub.Token == "" {
		log.Error("github_token_missing").
			Str("message", "Token GitHub não configurado").
			Send()
		return nil
	}

	enabledRepos := 0
	for _, repo := range cfg.GitHub.Repositories {
		if repo.Enabled {
			enabledRepos++
		}
	}

	log.Info("scanning_github_repositories").
		Int("total_repositories", len(cfg.GitHub.Repositories)).
		Int("enabled_repositories", enabledRepos).
		Send()

	for _, repo := range cfg.GitHub.Repositories {
		if repo.Enabled {
			log.Info("github_repository_configured").
				Str("repository", repo.Name).
				Int("priority", repo.Priority).
				Strs("paths", repo.Paths).
				Send()
		}
	}

	log.Info("operation_completed").
		Str("operation", "github_scan").
		Str("message", "Use 'privateer migrate github --dry-run' para escanear repositórios").
		Send()

	return nil
}
