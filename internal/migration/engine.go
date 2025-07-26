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
		if e.config.Settings.MultipleRegistries {
			for _, regConfig := range targetRegistries {
				wg.Add(1)
				go func(img *types.ImageInfo, regCfg types.RegistryConfig) {
					defer wg.Done()

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
	e.logger.Debug("migrating_image_to_registry_preserve_namespace").
		Str("image", image.Image).
		Str("registry", registryName).
		Str("namespace", image.Namespace).
		Str("resource", image.ResourceName).
		Send()

	reg, err := e.registryManager.GetRegistry(registryName)
	if err != nil {
		e.logger.Error("registry_not_found").
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

	if err := e.registryManager.ValidateImageDuplication(ctx, targetImage); err != nil {
		e.logger.Warn("image_duplication_detected").
			Str("image", targetImage).
			Str("registry", registryName).
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

	if err := reg.Login(ctx); err != nil {
		e.logger.Error("registry_login_failed").
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

	if err := reg.Copy(ctx, image.Image, targetImage); err != nil {
		e.logger.Error("image_copy_failed").
			Str("source", image.Image).
			Str("target", targetImage).
			Str("registry", registryName).
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

	e.logger.Info("image_migrated_preserve_namespace").
		Str("source", image.Image).
		Str("target", targetImage).
		Str("registry", registryName).
		Send()

	return &types.MigrationResult{
		Image:       image,
		TargetImage: targetImage,
		Registry:    registryName,
		Success:     true,
	}
}

func (e *Engine) generateTargetImageName(image *types.ImageInfo, reg registry.Registry) (string, error) {
	parsed := e.parseImageName(image.Image)
	targetRepository := parsed.FullRepository
	targetTag := parsed.Tag

	if parsed.Digest != "" {
		targetTag = fmt.Sprintf("%s@%s", targetTag, parsed.Digest)
	}

	switch reg.GetType() {
	case "docker":
		registryURL := e.getRegistryURL(reg.GetName())
		return fmt.Sprintf("%s/%s:%s", registryURL, targetRepository, targetTag), nil

	case "harbor":
		registryURL := e.getRegistryURL(reg.GetName())
		project := e.getHarborProject(reg.GetName())
		return fmt.Sprintf("%s/%s/%s:%s", registryURL, project, targetRepository, targetTag), nil

	case "ecr":
		ecrURL := e.getECRURL(reg.GetName())
		return fmt.Sprintf("%s/%s:%s", ecrURL, targetRepository, targetTag), nil

	case "ghcr":
		organization := e.getGHCROrganization(reg.GetName())
		return fmt.Sprintf("ghcr.io/%s/%s:%s", organization, targetRepository, targetTag), nil
	}

	return fmt.Sprintf("%s/%s:%s", reg.GetName(), targetRepository, targetTag), nil
}

func (e *Engine) parseImageName(imageName string) *ParsedImage {
	parsed := &ParsedImage{
		OriginalImage: imageName,
		Tag:           "latest",
	}

	workingImage := imageName

	if strings.Contains(workingImage, "@") {
		parts := strings.Split(workingImage, "@")
		workingImage = parts[0]
		parsed.Digest = parts[1]
	}

	if strings.Contains(workingImage, ":") {
		parts := strings.Split(workingImage, ":")
		workingImage = parts[0]
		parsed.Tag = parts[1]
	}

	parts := strings.Split(workingImage, "/")

	switch len(parts) {
	case 1:
		parsed.Registry = "docker.io"
		parsed.Namespace = "library"
		parsed.Repository = parts[0]
		parsed.FullRepository = fmt.Sprintf("library/%s", parts[0])

	case 2:
		if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
			parsed.Registry = parts[0]
			parsed.Namespace = ""
			parsed.Repository = parts[1]
			parsed.FullRepository = parts[1]
		} else {
			parsed.Registry = "docker.io"
			parsed.Namespace = parts[0]
			parsed.Repository = parts[1]
			parsed.FullRepository = fmt.Sprintf("%s/%s", parts[0], parts[1])
		}

	case 3:
		parsed.Registry = parts[0]
		parsed.Namespace = parts[1]
		parsed.Repository = parts[2]
		parsed.FullRepository = fmt.Sprintf("%s/%s", parts[1], parts[2])

	default:
		parsed.Registry = parts[0]
		parsed.Repository = parts[len(parts)-1]
		parsed.Namespace = strings.Join(parts[1:len(parts)-1], "/")
		parsed.FullRepository = strings.Join(parts[1:], "/")
	}

	if parsed.Registry == "index.docker.io" || parsed.Registry == "registry-1.docker.io" {
		parsed.Registry = "docker.io"
	}

	return parsed
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

	for _, regConfig := range e.config.Registries {
		if regConfig.Enabled {
			enabledRegistries = append(enabledRegistries, regConfig)
		}
	}

	if len(enabledRegistries) == 0 {
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
	} else if result.Skipped {
		summary.SkippedCount++
	} else {
		summary.FailureCount++
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

type ParsedImage struct {
	OriginalImage  string
	Registry       string
	Namespace      string
	Repository     string
	FullRepository string
	Tag            string
	Digest         string
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
