package migration

import (
	"context"
	"fmt"

	"github.com/kevinfinalboss/privateer/internal/registry"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

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

	if err := e.validateImageDuplication(ctx, targetImage, registryName, image); err != nil {
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

	if err := e.authenticateRegistry(ctx, reg, registryName); err != nil {
		return &types.MigrationResult{
			Image:    image,
			Registry: registryName,
			Success:  false,
			Error:    err,
		}
	}

	if err := e.copyImage(ctx, reg, image, targetImage, registryName); err != nil {
		return &types.MigrationResult{
			Image:       image,
			TargetImage: targetImage,
			Registry:    registryName,
			Success:     false,
			Error:       err,
		}
	}

	e.cleanupLocalImage(ctx, image.Image)

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

func (e *Engine) validateImageDuplication(ctx context.Context, targetImage, registryName string, image *types.ImageInfo) error {
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
		return err
	}

	e.logger.Debug("image_duplication_check_passed").
		Str("target_image", targetImage).
		Str("registry", registryName).
		Send()

	return nil
}

func (e *Engine) authenticateRegistry(ctx context.Context, reg registry.Registry, registryName string) error {
	e.logger.Debug("attempting_registry_login").
		Str("registry", registryName).
		Send()

	if err := reg.Login(ctx); err != nil {
		e.logger.Error("registry_login_failed").
			Str("registry", registryName).
			Err(err).
			Send()
		return err
	}

	e.logger.Debug("registry_login_successful").
		Str("registry", registryName).
		Send()

	return nil
}

func (e *Engine) copyImage(ctx context.Context, reg registry.Registry, image *types.ImageInfo, targetImage, registryName string) error {
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
		return err
	}

	e.logger.Info("image_copy_successful").
		Str("source", image.Image).
		Str("target", targetImage).
		Str("registry", registryName).
		Str("namespace", image.Namespace).
		Send()

	return nil
}

func (e *Engine) cleanupLocalImage(ctx context.Context, imageName string) error {
	e.logger.Debug("starting_local_image_cleanup").
		Str("source_image", imageName).
		Send()

	if err := e.registryManager.RemoveLocalImage(ctx, imageName); err != nil {
		e.logger.Warn("local_image_cleanup_failed").
			Str("image", imageName).
			Err(err).
			Send()
		return fmt.Errorf("falha ao remover imagem local %s: %w", imageName, err)
	}

	e.logger.Info("local_image_cleanup_successful").
		Str("image", imageName).
		Send()

	return nil
}
