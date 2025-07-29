package migration

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/internal/registry"
	"github.com/kevinfinalboss/privateer/internal/reporter"
	"github.com/kevinfinalboss/privateer/internal/webhook"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

type Engine struct {
	registryManager *registry.Manager
	logger          *logger.Logger
	config          *types.Config
	concurrency     int
	discordWebhook  *webhook.DiscordWebhook
	htmlReporter    *reporter.HTMLReporter
}

func NewEngine(registryManager *registry.Manager, logger *logger.Logger, cfg *types.Config) *Engine {
	concurrency := 3
	if cfg.Settings.Concurrency > 0 {
		concurrency = cfg.Settings.Concurrency
	}

	engine := &Engine{
		registryManager: registryManager,
		logger:          logger,
		config:          cfg,
		concurrency:     concurrency,
		htmlReporter:    reporter.NewHTMLReporter(logger),
	}

	if cfg.Webhooks.Discord.Enabled && cfg.Webhooks.Discord.URL != "" {
		engine.discordWebhook = webhook.NewDiscordWebhook(cfg.Webhooks.Discord, logger)
		logger.Info("discord_webhook_enabled").
			Str("url", maskWebhookURL(cfg.Webhooks.Discord.URL)).
			Send()
	}

	return engine
}

func (e *Engine) MigrateImages(ctx context.Context, images []*types.ImageInfo) (*types.MigrationSummary, error) {
	if len(images) == 0 {
		e.logger.Info("no_images_to_migrate").Send()
		return &types.MigrationSummary{}, nil
	}

	e.logger.Debug("migration_input_analysis").
		Int("total_input_images", len(images)).
		Send()

	for i, img := range images {
		e.logger.Debug("input_image_details").
			Int("index", i).
			Str("image", img.Image).
			Str("namespace", img.Namespace).
			Str("resource_name", img.ResourceName).
			Str("resource_type", img.ResourceType).
			Bool("is_public", img.IsPublic).
			Bool("is_init_container", img.IsInitContainer).
			Str("container", img.Container).
			Send()
	}

	targetRegistries := e.selectTargetRegistries()
	if len(targetRegistries) == 0 {
		err := fmt.Errorf("nenhum registry habilitado encontrado")
		e.logger.Error("no_enabled_registries").Send()

		if e.discordWebhook != nil {
			e.discordWebhook.SendError(ctx, err.Error(), "Seleção de Registries")
		}

		return nil, err
	}

	e.logger.Info("migration_started_preserve_namespace").
		Int("total_images", len(images)).
		Int("target_registries", len(targetRegistries)).
		Strs("registries", getRegistryNames(targetRegistries)).
		Bool("multiple_registries", e.config.Settings.MultipleRegistries).
		Int("concurrency", e.concurrency).
		Str("strategy", "preserve_full_namespace").
		Send()

	if e.discordWebhook != nil {
		err := e.discordWebhook.SendMigrationStart(ctx, len(images), getRegistryNames(targetRegistries), e.config.Settings.DryRun)
		if err != nil {
			e.logger.Warn("discord_webhook_failed").Err(err).Send()
		}
	}

	if e.config.Settings.DryRun {
		summary := e.dryRunMigration(images, targetRegistries)

		if e.discordWebhook != nil {
			err := e.discordWebhook.SendMigrationComplete(ctx, summary, true)
			if err != nil {
				e.logger.Warn("discord_webhook_failed").Err(err).Send()
			}
		}

		reportPath, err := e.htmlReporter.GenerateReport(summary, e.config, true)
		if err != nil {
			e.logger.Warn("html_report_failed").Err(err).Send()
		} else {
			e.logger.Info("html_report_ready").
				Str("path", reportPath).
				Str("message", "Relatório HTML de simulação gerado").
				Send()
		}

		return summary, nil
	}

	summary := &types.MigrationSummary{
		TotalImages: len(images),
		Results:     make([]*types.MigrationResult, 0),
	}

	semaphore := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, image := range images {
		e.logger.Debug("processing_image_for_migration").
			Str("image", image.Image).
			Str("namespace", image.Namespace).
			Bool("multiple_registries", e.config.Settings.MultipleRegistries).
			Send()

		if e.config.Settings.MultipleRegistries {
			for _, regConfig := range targetRegistries {
				wg.Add(1)
				go func(img *types.ImageInfo, regCfg types.RegistryConfig) {
					defer wg.Done()

					e.logger.Debug("starting_goroutine_for_multiple_registries").
						Str("image", img.Image).
						Str("registry", regCfg.Name).
						Send()

					semaphore <- struct{}{}
					defer func() { <-semaphore }()

					result := e.migrateImageToRegistry(ctx, img, regCfg.Name)

					mu.Lock()
					summary.Results = append(summary.Results, result)
					e.updateSummaryCounters(summary, result)
					mu.Unlock()
				}(image, regConfig)
			}
		} else {
			wg.Add(1)
			go func(img *types.ImageInfo) {
				defer wg.Done()

				e.logger.Debug("starting_goroutine_for_single_registry").
					Str("image", img.Image).
					Str("registry", targetRegistries[0].Name).
					Send()

				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				result := e.migrateImageToRegistry(ctx, img, targetRegistries[0].Name)

				mu.Lock()
				summary.Results = append(summary.Results, result)
				e.updateSummaryCounters(summary, result)
				mu.Unlock()
			}(image)
		}
	}

	wg.Wait()

	e.logger.Info("migration_completed").
		Int("total", summary.TotalImages).
		Int("success", summary.SuccessCount).
		Int("failures", summary.FailureCount).
		Int("skipped", summary.SkippedCount).
		Send()

	if e.discordWebhook != nil {
		err := e.discordWebhook.SendMigrationComplete(ctx, summary, false)
		if err != nil {
			e.logger.Warn("discord_webhook_failed").Err(err).Send()
		}
	}

	reportPath, err := e.htmlReporter.GenerateReport(summary, e.config, false)
	if err != nil {
		e.logger.Warn("html_report_failed").Err(err).Send()
	} else {
		e.logger.Info("html_report_ready").
			Str("path", reportPath).
			Str("message", "Relatório HTML de migração gerado").
			Send()
	}

	return summary, nil
}

func (e *Engine) migrateImageToRegistry(ctx context.Context, image *types.ImageInfo, registryName string) *types.MigrationResult {
	e.logger.Debug("starting_image_migration").
		Str("image", image.Image).
		Str("registry", registryName).
		Str("namespace", image.Namespace).
		Str("resource", image.ResourceName).
		Send()

	reg, err := e.registryManager.GetRegistry(registryName)
	if err != nil {
		e.logger.Error("registry_not_found").
			Str("registry", registryName).
			Str("image", image.Image).
			Err(err).
			Send()
		return &types.MigrationResult{
			Image:    image,
			Registry: registryName,
			Success:  false,
			Error:    err,
		}
	}

	e.logger.Debug("registry_found_successfully").
		Str("registry", registryName).
		Str("registry_type", reg.GetType()).
		Str("image", image.Image).
		Send()

	targetImage, err := e.generateTargetImageName(image, reg)
	if err != nil {
		e.logger.Error("target_image_generation_failed").
			Str("image", image.Image).
			Str("registry", registryName).
			Err(err).
			Send()
		return &types.MigrationResult{
			Image:    image,
			Registry: registryName,
			Success:  false,
			Error:    err,
		}
	}

	e.logger.Debug("target_image_generated").
		Str("source_image", image.Image).
		Str("target_image", targetImage).
		Str("registry", registryName).
		Send()

	e.logger.Debug("checking_image_duplication").
		Str("target_image", targetImage).
		Str("registry", registryName).
		Send()

	if err := e.registryManager.ValidateImageDuplication(ctx, targetImage); err != nil {
		e.logger.Warn("image_duplication_detected").
			Str("source_image", image.Image).
			Str("target_image", targetImage).
			Str("registry", registryName).
			Str("namespace", image.Namespace).
			Str("resource", image.ResourceName).
			Str("skip_reason", "Imagem já existe no registry").
			Err(err).
			Send()
		return &types.MigrationResult{
			Image:       image,
			TargetImage: targetImage,
			Registry:    registryName,
			Success:     false,
			Skipped:     true,
			Reason:      "Imagem já existe no registry",
			Error:       err,
		}
	}

	e.logger.Debug("image_duplication_check_passed").
		Str("target_image", targetImage).
		Str("registry", registryName).
		Send()

	e.logger.Debug("attempting_registry_login").
		Str("registry", registryName).
		Send()

	if err := reg.Login(ctx); err != nil {
		e.logger.Error("registry_login_failed").
			Str("registry", registryName).
			Str("image", image.Image).
			Err(err).
			Send()
		return &types.MigrationResult{
			Image:    image,
			Registry: registryName,
			Success:  false,
			Error:    err,
		}
	}

	e.logger.Debug("registry_login_successful").
		Str("registry", registryName).
		Send()

	e.logger.Info("starting_image_copy").
		Str("source", image.Image).
		Str("target", targetImage).
		Str("registry", registryName).
		Str("namespace", image.Namespace).
		Send()

	if err := reg.Copy(ctx, image.Image, targetImage); err != nil {
		e.logger.Error("image_copy_failed").
			Str("source", image.Image).
			Str("target", targetImage).
			Str("registry", registryName).
			Str("namespace", image.Namespace).
			Err(err).
			Send()
		return &types.MigrationResult{
			Image:       image,
			TargetImage: targetImage,
			Registry:    registryName,
			Success:     false,
			Error:       err,
		}
	}

	e.logger.Info("image_copy_successful").
		Str("source", image.Image).
		Str("target", targetImage).
		Str("registry", registryName).
		Str("namespace", image.Namespace).
		Send()

	e.logger.Debug("starting_local_image_cleanup").
		Str("source_image", image.Image).
		Send()

	if err := e.cleanupLocalImage(ctx, image.Image); err != nil {
		e.logger.Warn("local_image_cleanup_failed").
			Str("image", image.Image).
			Err(err).
			Send()
	} else {
		e.logger.Info("local_image_cleanup_successful").
			Str("image", image.Image).
			Send()
	}

	e.logger.Info("image_migrated_preserve_namespace").
		Str("source", image.Image).
		Str("target", targetImage).
		Str("registry", registryName).
		Str("namespace", image.Namespace).
		Send()

	return &types.MigrationResult{
		Image:       image,
		TargetImage: targetImage,
		Registry:    registryName,
		Success:     true,
	}
}

func (e *Engine) cleanupLocalImage(ctx context.Context, imageName string) error {
	e.logger.Debug("executing_local_image_removal").
		Str("image", imageName).
		Send()

	if err := e.registryManager.RemoveLocalImage(ctx, imageName); err != nil {
		e.logger.Error("local_image_removal_failed").
			Str("image", imageName).
			Err(err).
			Send()
		return fmt.Errorf("falha ao remover imagem local %s: %w", imageName, err)
	}

	e.logger.Debug("local_image_removal_completed").
		Str("image", imageName).
		Send()

	return nil
}

func (e *Engine) generateTargetImageName(image *types.ImageInfo, reg registry.Registry) (string, error) {
	e.logger.Debug("parsing_image_name").
		Str("image", image.Image).
		Str("registry_type", reg.GetType()).
		Send()

	parsed := types.ParseImageName(image.Image)

	e.logger.Debug("image_parsing_result").
		Str("original_image", parsed.OriginalImage).
		Str("registry", parsed.Registry).
		Str("namespace", parsed.Namespace).
		Str("repository", parsed.Repository).
		Str("full_repository", parsed.FullRepository).
		Str("tag", parsed.Tag).
		Str("digest", parsed.Digest).
		Send()

	targetRepository := parsed.FullRepository
	targetTag := parsed.Tag

	if parsed.Digest != "" {
		targetTag = fmt.Sprintf("%s@%s", targetTag, parsed.Digest)
		e.logger.Debug("digest_detected_in_target").
			Str("target_tag", targetTag).
			Send()
	}

	var targetImage string
	var err error

	switch reg.GetType() {
	case "docker":
		registryURL := e.getRegistryURL(reg.GetName())
		targetImage = fmt.Sprintf("%s/%s:%s", registryURL, targetRepository, targetTag)
		e.logger.Debug("docker_target_image_generated").
			Str("registry_url", registryURL).
			Str("target_image", targetImage).
			Send()

	case "harbor":
		registryURL := e.getRegistryURL(reg.GetName())
		project := e.getHarborProject(reg.GetName())
		targetImage = fmt.Sprintf("%s/%s/%s:%s", registryURL, project, targetRepository, targetTag)
		e.logger.Debug("harbor_target_image_generated").
			Str("registry_url", registryURL).
			Str("project", project).
			Str("target_image", targetImage).
			Send()

	case "ecr":
		ecrURL := e.getECRURL(reg.GetName())
		targetImage = fmt.Sprintf("%s/%s:%s", ecrURL, targetRepository, targetTag)
		e.logger.Debug("ecr_target_image_generated").
			Str("ecr_url", ecrURL).
			Str("target_image", targetImage).
			Send()

	case "ghcr":
		organization := e.getGHCROrganization(reg.GetName())
		targetImage = fmt.Sprintf("ghcr.io/%s/%s:%s", organization, targetRepository, targetTag)
		e.logger.Debug("ghcr_target_image_generated").
			Str("organization", organization).
			Str("target_image", targetImage).
			Send()

	default:
		targetImage = fmt.Sprintf("%s/%s:%s", reg.GetName(), targetRepository, targetTag)
		e.logger.Debug("default_target_image_generated").
			Str("target_image", targetImage).
			Send()
	}

	if targetImage == "" {
		err = fmt.Errorf("falha ao gerar nome da imagem de destino para %s", image.Image)
		e.logger.Error("target_image_generation_empty").
			Str("source_image", image.Image).
			Str("registry_type", reg.GetType()).
			Send()
		return "", err
	}

	return targetImage, nil
}

func (e *Engine) getRegistryURL(registryName string) string {
	for _, regConfig := range e.config.Registries {
		if regConfig.Name == registryName {
			url := regConfig.URL
			if strings.HasPrefix(url, "http://") {
				url = strings.TrimPrefix(url, "http://")
			} else if strings.HasPrefix(url, "https://") {
				url = strings.TrimPrefix(url, "https://")
			}
			return url
		}
	}
	return registryName
}

func (e *Engine) getHarborProject(registryName string) string {
	for _, regConfig := range e.config.Registries {
		if regConfig.Name == registryName && regConfig.Project != "" {
			return regConfig.Project
		}
	}
	return "library"
}

func (e *Engine) getECRURL(registryName string) string {
	for _, regConfig := range e.config.Registries {
		if regConfig.Name == registryName && regConfig.Type == "ecr" {
			if regConfig.AccountID != "" {
				return fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", regConfig.AccountID, regConfig.Region)
			}
		}
	}
	return registryName
}

func (e *Engine) getGHCROrganization(registryName string) string {
	for _, regConfig := range e.config.Registries {
		if regConfig.Name == registryName {
			if regConfig.Project != "" {
				return regConfig.Project
			}
			return regConfig.Username
		}
	}
	return "unknown"
}

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

func (e *Engine) updateSummaryCounters(summary *types.MigrationSummary, result *types.MigrationResult) {
	if result.Success {
		summary.SuccessCount++
		e.logger.Debug("summary_counter_updated").
			Str("type", "success").
			Int("new_count", summary.SuccessCount).
			Send()
	} else if result.Skipped {
		summary.SkippedCount++
		e.logger.Debug("summary_counter_updated").
			Str("type", "skipped").
			Int("new_count", summary.SkippedCount).
			Str("reason", result.Reason).
			Send()
	} else {
		summary.FailureCount++
		e.logger.Debug("summary_counter_updated").
			Str("type", "failure").
			Int("new_count", summary.FailureCount).
			Send()
		if result.Error != nil {
			summary.Errors = append(summary.Errors, result.Error)
		}
	}
}

func (e *Engine) dryRunMigration(images []*types.ImageInfo, targetRegistries []types.RegistryConfig) *types.MigrationSummary {
	e.logger.Info("dry_run_migration_preserve_namespace").
		Int("total_images", len(images)).
		Int("target_registries", len(targetRegistries)).
		Send()

	var totalOperations int
	if e.config.Settings.MultipleRegistries {
		totalOperations = len(images) * len(targetRegistries)
	} else {
		totalOperations = len(images)
	}

	summary := &types.MigrationSummary{
		TotalImages:  len(images),
		SuccessCount: totalOperations,
		Results:      make([]*types.MigrationResult, 0, totalOperations),
	}

	for _, image := range images {
		if e.config.Settings.MultipleRegistries {
			for _, regConfig := range targetRegistries {
				reg, err := e.registryManager.GetRegistry(regConfig.Name)
				if err != nil {
					e.logger.Error("registry_not_found_dry_run").
						Str("registry", regConfig.Name).
						Err(err).
						Send()
					continue
				}

				targetImage, err := e.generateTargetImageName(image, reg)
				if err != nil {
					e.logger.Error("target_image_generation_failed_dry_run").
						Str("registry", regConfig.Name).
						Err(err).
						Send()
					continue
				}

				e.logger.Info("dry_run_would_migrate_preserve_namespace").
					Str("source", image.Image).
					Str("target", targetImage).
					Str("registry", regConfig.Name).
					Int("priority", regConfig.Priority).
					Send()

				summary.Results = append(summary.Results, &types.MigrationResult{
					Image:       image,
					TargetImage: targetImage,
					Registry:    regConfig.Name,
					Success:     true,
				})
			}
		} else {
			reg, err := e.registryManager.GetRegistry(targetRegistries[0].Name)
			if err != nil {
				e.logger.Error("registry_not_found_dry_run").
					Str("registry", targetRegistries[0].Name).
					Err(err).
					Send()
				continue
			}

			targetImage, err := e.generateTargetImageName(image, reg)
			if err != nil {
				e.logger.Error("target_image_generation_failed_dry_run").
					Str("registry", targetRegistries[0].Name).
					Err(err).
					Send()
				continue
			}

			e.logger.Info("dry_run_would_migrate_preserve_namespace").
				Str("source", image.Image).
				Str("target", targetImage).
				Str("registry", targetRegistries[0].Name).
				Int("priority", targetRegistries[0].Priority).
				Str("mode", "highest_priority_only").
				Send()

			summary.Results = append(summary.Results, &types.MigrationResult{
				Image:       image,
				TargetImage: targetImage,
				Registry:    targetRegistries[0].Name,
				Success:     true,
			})
		}
	}

	return summary
}

func getRegistryNames(registries []types.RegistryConfig) []string {
	names := make([]string, len(registries))
	for i, reg := range registries {
		names[i] = fmt.Sprintf("%s (priority: %d)", reg.Name, reg.Priority)
	}
	return names
}

func maskWebhookURL(url string) string {
	if len(url) < 20 {
		return "***"
	}
	return url[:20] + "***"
}
