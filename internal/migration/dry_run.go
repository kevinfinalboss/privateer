package migration

import (
	"github.com/kevinfinalboss/privateer/pkg/types"
)

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
			e.processDryRunForMultipleRegistries(image, targetRegistries, summary)
		} else {
			e.processDryRunForSingleRegistry(image, targetRegistries[0], summary)
		}
	}

	return summary
}

func (e *Engine) processDryRunForMultipleRegistries(image *types.ImageInfo, targetRegistries []types.RegistryConfig, summary *types.MigrationSummary) {
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
}

func (e *Engine) processDryRunForSingleRegistry(image *types.ImageInfo, registry types.RegistryConfig, summary *types.MigrationSummary) {
	reg, err := e.registryManager.GetRegistry(registry.Name)
	if err != nil {
		e.logger.Error("registry_not_found_dry_run").
			Str("registry", registry.Name).
			Err(err).
			Send()
		return
	}

	targetImage, err := e.generateTargetImageName(image, reg)
	if err != nil {
		e.logger.Error("target_image_generation_failed_dry_run").
			Str("registry", registry.Name).
			Err(err).
			Send()
		return
	}

	e.logger.Info("dry_run_would_migrate_preserve_namespace").
		Str("source", image.Image).
		Str("target", targetImage).
		Str("registry", registry.Name).
		Int("priority", registry.Priority).
		Str("mode", "highest_priority_only").
		Send()

	summary.Results = append(summary.Results, &types.MigrationResult{
		Image:       image,
		TargetImage: targetImage,
		Registry:    registry.Name,
		Success:     true,
	})
}
