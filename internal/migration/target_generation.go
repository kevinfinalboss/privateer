package migration

import (
	"fmt"
	"strings"

	"github.com/kevinfinalboss/privateer/internal/registry"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

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
		targetImage = e.generateDockerTargetImage(reg.GetName(), targetRepository, targetTag)
	case "harbor":
		targetImage = e.generateHarborTargetImage(reg.GetName(), targetRepository, targetTag)
	case "ecr":
		targetImage = e.generateECRTargetImage(reg.GetName(), targetRepository, targetTag)
	case "ghcr":
		targetImage = e.generateGHCRTargetImage(reg.GetName(), targetRepository, targetTag)
	default:
		targetImage = e.generateDefaultTargetImage(reg.GetName(), targetRepository, targetTag)
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

func (e *Engine) generateDockerTargetImage(registryName, targetRepository, targetTag string) string {
	registryURL := e.getRegistryURL(registryName)
	targetImage := fmt.Sprintf("%s/%s:%s", registryURL, targetRepository, targetTag)

	e.logger.Debug("docker_target_image_generated").
		Str("registry_url", registryURL).
		Str("target_image", targetImage).
		Send()

	return targetImage
}

func (e *Engine) generateHarborTargetImage(registryName, targetRepository, targetTag string) string {
	registryURL := e.getRegistryURL(registryName)
	project := e.getHarborProject(registryName)
	targetImage := fmt.Sprintf("%s/%s/%s:%s", registryURL, project, targetRepository, targetTag)

	e.logger.Debug("harbor_target_image_generated").
		Str("registry_url", registryURL).
		Str("project", project).
		Str("target_image", targetImage).
		Send()

	return targetImage
}

func (e *Engine) generateECRTargetImage(registryName, targetRepository, targetTag string) string {
	ecrURL := e.getECRURL(registryName)
	targetImage := fmt.Sprintf("%s/%s:%s", ecrURL, targetRepository, targetTag)

	e.logger.Debug("ecr_target_image_generated").
		Str("ecr_url", ecrURL).
		Str("target_image", targetImage).
		Send()

	return targetImage
}

func (e *Engine) generateGHCRTargetImage(registryName, targetRepository, targetTag string) string {
	organization := e.getGHCROrganization(registryName)
	targetImage := fmt.Sprintf("ghcr.io/%s/%s:%s", organization, targetRepository, targetTag)

	e.logger.Debug("ghcr_target_image_generated").
		Str("organization", organization).
		Str("target_image", targetImage).
		Send()

	return targetImage
}

func (e *Engine) generateDefaultTargetImage(registryName, targetRepository, targetTag string) string {
	targetImage := fmt.Sprintf("%s/%s:%s", registryName, targetRepository, targetTag)

	e.logger.Debug("default_target_image_generated").
		Str("target_image", targetImage).
		Send()

	return targetImage
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
