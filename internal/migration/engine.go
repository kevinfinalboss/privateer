package migration

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/internal/registry"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

type Engine struct {
	registryManager *registry.Manager
	logger          *logger.Logger
	config          *types.Config
	concurrency     int
}

type MigrationResult struct {
	Image       *types.ImageInfo
	TargetImage string
	Registry    string
	Success     bool
	Error       error
	Skipped     bool
	Reason      string
}

type MigrationSummary struct {
	TotalImages  int
	SuccessCount int
	FailureCount int
	SkippedCount int
	Results      []*MigrationResult
	Errors       []error
}

func NewEngine(registryManager *registry.Manager, logger *logger.Logger, cfg *types.Config) *Engine {
	concurrency := 3
	if cfg.Settings.Concurrency > 0 {
		concurrency = cfg.Settings.Concurrency
	}

	return &Engine{
		registryManager: registryManager,
		logger:          logger,
		config:          cfg,
		concurrency:     concurrency,
	}
}

func (e *Engine) MigrateImages(ctx context.Context, images []*types.ImageInfo) (*MigrationSummary, error) {
	if len(images) == 0 {
		e.logger.Info("no_images_to_migrate").Send()
		return &MigrationSummary{}, nil
	}

	enabledRegistries := e.getEnabledRegistries()
	if len(enabledRegistries) == 0 {
		err := fmt.Errorf("nenhum registry habilitado encontrado")
		e.logger.Error("no_enabled_registries").Send()
		return nil, err
	}

	e.logger.Info("migration_started").
		Int("total_images", len(images)).
		Int("enabled_registries", len(enabledRegistries)).
		Strs("registries", enabledRegistries).
		Int("concurrency", e.concurrency).
		Send()

	if e.config.Settings.DryRun {
		return e.dryRunMigration(images, enabledRegistries), nil
	}

	summary := &MigrationSummary{
		TotalImages: len(images),
		Results:     make([]*MigrationResult, 0, len(images)*len(enabledRegistries)),
	}

	semaphore := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, image := range images {
		for _, registryName := range enabledRegistries {
			wg.Add(1)
			go func(img *types.ImageInfo, regName string) {
				defer wg.Done()

				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				result := e.migrateImageToRegistry(ctx, img, regName)

				mu.Lock()
				summary.Results = append(summary.Results, result)
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
				mu.Unlock()
			}(image, registryName)
		}
	}

	wg.Wait()

	e.logger.Info("migration_completed").
		Int("total", summary.TotalImages).
		Int("success", summary.SuccessCount).
		Int("failures", summary.FailureCount).
		Int("skipped", summary.SkippedCount).
		Send()

	return summary, nil
}

func (e *Engine) migrateImageToRegistry(ctx context.Context, image *types.ImageInfo, registryName string) *MigrationResult {
	e.logger.Debug("migrating_image_to_registry").
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
		return &MigrationResult{
			Image:    image,
			Registry: registryName,
			Success:  false,
			Error:    err,
		}
	}

	targetImage := e.generateTargetImageName(image, reg)

	if err := e.registryManager.ValidateImageDuplication(ctx, targetImage); err != nil {
		e.logger.Warn("image_duplication_detected").
			Str("image", targetImage).
			Str("registry", registryName).
			Err(err).
			Send()
		return &MigrationResult{
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
		return &MigrationResult{
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
		return &MigrationResult{
			Image:       image,
			TargetImage: targetImage,
			Registry:    registryName,
			Success:     false,
			Error:       err,
		}
	}

	e.logger.Info("image_migrated").
		Str("source", image.Image).
		Str("target", targetImage).
		Str("registry", registryName).
		Send()

	return &MigrationResult{
		Image:       image,
		TargetImage: targetImage,
		Registry:    registryName,
		Success:     true,
	}
}

func (e *Engine) getEnabledRegistries() []string {
	var enabledRegistries []string

	for _, regConfig := range e.config.Registries {
		if regConfig.Enabled {
			enabledRegistries = append(enabledRegistries, regConfig.Name)
		}
	}

	return enabledRegistries
}

func (e *Engine) generateTargetImageName(image *types.ImageInfo, reg registry.Registry) string {
	parts := strings.Split(image.Image, "/")
	imageName := parts[len(parts)-1]

	if !strings.Contains(imageName, ":") {
		imageName += ":latest"
	}

	switch reg.GetType() {
	case "docker":
		registryURL := e.getRegistryURL(reg.GetName())
		return fmt.Sprintf("%s/%s", registryURL, imageName)
	case "harbor":
		registryURL := e.getRegistryURL(reg.GetName())
		project := e.getHarborProject(reg.GetName())
		return fmt.Sprintf("%s/%s/%s", registryURL, project, imageName)
	case "ecr":
		ecrURL := e.getECRURL(reg.GetName())
		return fmt.Sprintf("%s/%s", ecrURL, imageName)
	case "ghcr":
		organization := e.getGHCROrganization(reg.GetName())
		return fmt.Sprintf("ghcr.io/%s/%s", organization, imageName)
	}

	return fmt.Sprintf("%s/%s", reg.GetName(), imageName)
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
		if regConfig.Name == registryName {
			return fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", regConfig.AccountID, regConfig.Region)
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

func (e *Engine) dryRunMigration(images []*types.ImageInfo, enabledRegistries []string) *MigrationSummary {
	e.logger.Info("dry_run_migration").
		Int("total_images", len(images)).
		Int("enabled_registries", len(enabledRegistries)).
		Send()

	totalOperations := len(images) * len(enabledRegistries)
	summary := &MigrationSummary{
		TotalImages:  len(images),
		SuccessCount: totalOperations,
		Results:      make([]*MigrationResult, 0, totalOperations),
	}

	for _, image := range images {
		for _, registryName := range enabledRegistries {
			reg, err := e.registryManager.GetRegistry(registryName)
			if err != nil {
				e.logger.Error("registry_not_found_dry_run").
					Str("registry", registryName).
					Err(err).
					Send()
				continue
			}

			targetImage := e.generateTargetImageName(image, reg)

			e.logger.Info("dry_run_would_migrate").
				Str("source", image.Image).
				Str("target", targetImage).
				Str("registry", registryName).
				Send()

			summary.Results = append(summary.Results, &MigrationResult{
				Image:       image,
				TargetImage: targetImage,
				Registry:    registryName,
				Success:     true,
			})
		}
	}

	return summary
}
