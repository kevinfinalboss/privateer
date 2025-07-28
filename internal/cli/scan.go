package cli

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/kevinfinalboss/privateer/internal/kubernetes"
	"github.com/kevinfinalboss/privateer/internal/registry"
	"github.com/kevinfinalboss/privateer/pkg/types"
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

type ScanResult struct {
	PublicImages       []*types.ImageInfo
	AvailableInPrivate map[string][]string
	NotAvailableImages []*types.ImageInfo
	RegistryStats      map[string]int
	TotalScanned       int
	TotalPublic        int
	TotalAvailable     int
	ScanDuration       time.Duration
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
	startTime := time.Now()

	client, err := kubernetes.NewClient(cfg, log)
	if err != nil {
		return err
	}

	registryManager := registry.NewManager(log)
	for _, regConfig := range cfg.Registries {
		if err := registryManager.AddRegistry(&regConfig); err != nil {
			log.Warn("registry_add_failed").
				Str("registry", regConfig.Name).
				Err(err).
				Send()
			continue
		}
	}

	if registryManager.GetRegistryCount() == 0 {
		log.Warn("no_registries_configured").
			Str("message", "Nenhum registry privado configurado").
			Send()
		return fmt.Errorf("nenhum registry privado configurado")
	}

	log.Info("registry_health_check_starting").
		Int("registries", registryManager.GetRegistryCount()).
		Send()

	ctx := context.Background()
	if err := registryManager.HealthCheck(ctx); err != nil {
		log.Warn("registry_health_check_issues").
			Err(err).
			Send()
	}

	namespaces, err := client.GetNamespaces()
	if err != nil {
		log.Error("operation_failed").Err(err).Send()
		return err
	}

	log.Info("scanning_cluster").
		Int("namespace_count", len(namespaces)).
		Strs("namespaces", namespaces).
		Int("configured_registries", registryManager.GetRegistryCount()).
		Send()

	scanner := kubernetes.NewScanner(client, log, cfg)
	result := &ScanResult{
		PublicImages:       make([]*types.ImageInfo, 0),
		AvailableInPrivate: make(map[string][]string),
		NotAvailableImages: make([]*types.ImageInfo, 0),
		RegistryStats:      make(map[string]int),
	}

	for _, namespace := range namespaces {
		publicImages, err := scanner.ScanNamespace(namespace)
		if err != nil {
			log.Error("operation_failed").
				Str("namespace", namespace).
				Err(err).
				Send()
			continue
		}

		result.PublicImages = append(result.PublicImages, publicImages...)
	}

	result.TotalScanned = len(result.PublicImages)
	result.TotalPublic = len(result.PublicImages)

	log.Info("validating_images_in_registries").
		Int("public_images", len(result.PublicImages)).
		Send()

	validatedMap, err := registryManager.ValidateImagesBatch(ctx, result.PublicImages, cfg)
	if err != nil {
		log.Warn("batch_validation_partial_failure").
			Err(err).
			Send()
	}

	for _, image := range result.PublicImages {
		privateImage, registryName, err := registryManager.FindImageInRegistries(ctx, image, cfg)
		if err != nil {
			result.NotAvailableImages = append(result.NotAvailableImages, image)
			log.Debug("image_not_found_in_private").
				Str("public_image", image.Image).
				Str("namespace", image.Namespace).
				Str("resource", image.ResourceName).
				Send()
			continue
		}

		if result.AvailableInPrivate[image.Image] == nil {
			result.AvailableInPrivate[image.Image] = make([]string, 0)
		}
		result.AvailableInPrivate[image.Image] = append(result.AvailableInPrivate[image.Image], fmt.Sprintf("%s (%s)", privateImage, registryName))
		result.RegistryStats[registryName]++
		result.TotalAvailable++
	}

	result.ScanDuration = time.Since(startTime)

	printScanSummary(result, validatedMap)
	printDetailedResults(result)
	printRegistryStats(result)
	printRecommendations(result)

	log.Info("operation_completed").
		Str("operation", "cluster_scan").
		Int("total_scanned", result.TotalScanned).
		Int("available_in_private", result.TotalAvailable).
		Int("not_available", len(result.NotAvailableImages)).
		Str("duration", result.ScanDuration.String()).
		Send()

	return nil
}

func printScanSummary(result *ScanResult, validatedMap map[string]string) {
	log.Info("scan_summary").
		Str("separator", "===========================================").
		Send()

	log.Info("scan_results_summary").
		Int("total_images_scanned", result.TotalScanned).
		Int("public_images_found", result.TotalPublic).
		Int("available_in_private", result.TotalAvailable).
		Int("not_available_in_private", len(result.NotAvailableImages)).
		Int("validated_from_batch", len(validatedMap)).
		Str("scan_duration", result.ScanDuration.String()).
		Send()

	if result.TotalPublic > 0 {
		availabilityPercentage := float64(result.TotalAvailable) / float64(result.TotalPublic) * 100
		log.Info("availability_percentage").
			Float64("percentage", availabilityPercentage).
			Send()
	}
}

func printDetailedResults(result *ScanResult) {
	if len(result.AvailableInPrivate) > 0 {
		log.Info("images_available_in_private").
			Str("separator", "-------------------------------------------").
			Send()

		sortedImages := make([]string, 0, len(result.AvailableInPrivate))
		for image := range result.AvailableInPrivate {
			sortedImages = append(sortedImages, image)
		}
		sort.Strings(sortedImages)

		for _, image := range sortedImages {
			registries := result.AvailableInPrivate[image]
			log.Info("image_available").
				Str("public_image", image).
				Strs("private_locations", registries).
				Int("registry_count", len(registries)).
				Send()
		}
	}

	if len(result.NotAvailableImages) > 0 {
		log.Info("images_not_available_in_private").
			Str("separator", "-------------------------------------------").
			Send()

		imageMap := make(map[string][]*types.ImageInfo)
		for _, image := range result.NotAvailableImages {
			imageMap[image.Image] = append(imageMap[image.Image], image)
		}

		sortedImages := make([]string, 0, len(imageMap))
		for image := range imageMap {
			sortedImages = append(sortedImages, image)
		}
		sort.Strings(sortedImages)

		for _, imageName := range sortedImages {
			images := imageMap[imageName]
			usageCount := len(images)

			namespaces := make([]string, 0)
			resources := make([]string, 0)

			for _, img := range images {
				namespaces = append(namespaces, img.Namespace)
				resources = append(resources, fmt.Sprintf("%s/%s", img.ResourceType, img.ResourceName))
			}

			log.Info("image_not_available").
				Str("public_image", imageName).
				Int("usage_count", usageCount).
				Strs("namespaces", namespaces).
				Strs("resources", resources).
				Send()
		}
	}
}

func printRegistryStats(result *ScanResult) {
	if len(result.RegistryStats) > 0 {
		log.Info("registry_statistics").
			Str("separator", "-------------------------------------------").
			Send()

		type registryStat struct {
			name  string
			count int
		}

		stats := make([]registryStat, 0, len(result.RegistryStats))
		for name, count := range result.RegistryStats {
			stats = append(stats, registryStat{name: name, count: count})
		}

		sort.Slice(stats, func(i, j int) bool {
			return stats[i].count > stats[j].count
		})

		for _, stat := range stats {
			percentage := float64(stat.count) / float64(result.TotalAvailable) * 100
			log.Info("registry_stat").
				Str("registry", stat.name).
				Int("image_count", stat.count).
				Float64("percentage", percentage).
				Send()
		}
	}
}

func printRecommendations(result *ScanResult) {
	log.Info("recommendations").
		Str("separator", "===========================================").
		Send()

	if len(result.NotAvailableImages) > 0 {
		log.Info("migration_recommendation").
			Str("message", "Para migrar as imagens não disponíveis").
			Str("command", "privateer migrate cluster --dry-run").
			Send()

		uniqueImages := make(map[string]bool)
		for _, img := range result.NotAvailableImages {
			uniqueImages[img.Image] = true
		}

		log.Info("migration_candidates").
			Int("unique_images_to_migrate", len(uniqueImages)).
			Send()
	}

	if result.TotalAvailable > 0 {
		log.Info("gitops_recommendation").
			Str("message", "Para atualizar repositórios com imagens disponíveis").
			Str("command", "privateer migrate github --dry-run").
			Send()
	}

	if len(result.RegistryStats) > 1 {
		log.Info("consolidation_recommendation").
			Str("message", "Considere consolidar imagens em registries menos utilizados").
			Send()
	}
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
