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
}

type MigrationSummary struct {
	TotalImages  int
	SuccessCount int
	FailureCount int
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

	e.logger.Info("migration_started").
		Int("total_images", len(images)).
		Int("concurrency", e.concurrency).
		Send()

	if e.config.Settings.DryRun {
		return e.dryRunMigration(images), nil
	}

	summary := &MigrationSummary{
		TotalImages: len(images),
		Results:     make([]*MigrationResult, 0, len(images)),
	}

	semaphore := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, image := range images {
		wg.Add(1)
		go func(img *types.ImageInfo) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := e.migrateImage(ctx, img)

			mu.Lock()
			summary.Results = append(summary.Results, result)
			if result.Success {
				summary.SuccessCount++
			} else {
				summary.FailureCount++
				if result.Error != nil {
					summary.Errors = append(summary.Errors, result.Error)
				}
			}
			mu.Unlock()
		}(image)
	}

	wg.Wait()

	e.logger.Info("migration_completed").
		Int("total", summary.TotalImages).
		Int("success", summary.SuccessCount).
		Int("failures", summary.FailureCount).
		Send()

	return summary, nil
}

func (e *Engine) migrateImage(ctx context.Context, image *types.ImageInfo) *MigrationResult {
	e.logger.Debug("migrating_image").
		Str("image", image.Image).
		Str("namespace", image.Namespace).
		Str("resource", image.ResourceName).
		Send()

	targetRegistry := e.selectTargetRegistry(image)
	if targetRegistry == "" {
		err := fmt.Errorf("nenhum registry de destino configurado")
		e.logger.Error("no_target_registry").
			Str("image", image.Image).
			Send()
		return &MigrationResult{
			Image:   image,
			Success: false,
			Error:   err,
		}
	}

	reg, err := e.registryManager.GetRegistry(targetRegistry)
	if err != nil {
		e.logger.Error("registry_not_found").
			Str("registry", targetRegistry).
			Err(err).
			Send()
		return &MigrationResult{
			Image:    image,
			Registry: targetRegistry,
			Success:  false,
			Error:    err,
		}
	}

	if err := reg.Login(ctx); err != nil {
		e.logger.Error("registry_login_failed").
			Str("registry", targetRegistry).
			Err(err).
			Send()
		return &MigrationResult{
			Image:    image,
			Registry: targetRegistry,
			Success:  false,
			Error:    err,
		}
	}

	targetImage := e.generateTargetImageName(image, reg)

	if err := reg.Copy(ctx, image.Image, targetImage); err != nil {
		e.logger.Error("image_copy_failed").
			Str("source", image.Image).
			Str("target", targetImage).
			Err(err).
			Send()
		return &MigrationResult{
			Image:       image,
			TargetImage: targetImage,
			Registry:    targetRegistry,
			Success:     false,
			Error:       err,
		}
	}

	e.logger.Info("image_migrated").
		Str("source", image.Image).
		Str("target", targetImage).
		Str("registry", targetRegistry).
		Send()

	return &MigrationResult{
		Image:       image,
		TargetImage: targetImage,
		Registry:    targetRegistry,
		Success:     true,
	}
}

func (e *Engine) selectTargetRegistry(_ *types.ImageInfo) string {
	registries := e.registryManager.ListRegistries()
	if len(registries) == 0 {
		return ""
	}

	return registries[0]
}

func (e *Engine) generateTargetImageName(image *types.ImageInfo, reg registry.Registry) string {
	parts := strings.Split(image.Image, "/")
	imageName := parts[len(parts)-1]

	if !strings.Contains(imageName, ":") {
		imageName += ":latest"
	}

	switch reg.GetType() {
	case "docker":
		if dockerReg, ok := reg.(*registry.DockerRegistry); ok {
			registryURL := dockerReg.URL
			if strings.HasPrefix(registryURL, "http://") {
				registryURL = strings.TrimPrefix(registryURL, "http://")
			} else if strings.HasPrefix(registryURL, "https://") {
				registryURL = strings.TrimPrefix(registryURL, "https://")
			}
			return fmt.Sprintf("%s/%s", registryURL, imageName)
		}
	case "harbor":
		if harborReg, ok := reg.(*registry.HarborRegistry); ok {
			registryURL := harborReg.URL
			if strings.HasPrefix(registryURL, "http://") {
				registryURL = strings.TrimPrefix(registryURL, "http://")
			} else if strings.HasPrefix(registryURL, "https://") {
				registryURL = strings.TrimPrefix(registryURL, "https://")
			}
			return fmt.Sprintf("%s/library/%s", registryURL, imageName)
		}
	}

	return fmt.Sprintf("%s/%s", reg.GetName(), imageName)
}

func (e *Engine) dryRunMigration(images []*types.ImageInfo) *MigrationSummary {
	e.logger.Info("dry_run_migration").
		Int("total_images", len(images)).
		Send()

	summary := &MigrationSummary{
		TotalImages:  len(images),
		SuccessCount: len(images),
		Results:      make([]*MigrationResult, 0, len(images)),
	}

	registries := e.registryManager.ListRegistries()
	targetRegistry := ""
	if len(registries) > 0 {
		targetRegistry = registries[0]
	}

	for _, image := range images {
		targetImage := fmt.Sprintf("%s/%s", targetRegistry,
			strings.TrimPrefix(image.Image, strings.Split(image.Image, "/")[0]+"/"))

		e.logger.Info("dry_run_would_migrate").
			Str("source", image.Image).
			Str("target", targetImage).
			Str("registry", targetRegistry).
			Send()

		summary.Results = append(summary.Results, &MigrationResult{
			Image:       image,
			TargetImage: targetImage,
			Registry:    targetRegistry,
			Success:     true,
		})
	}

	return summary
}
