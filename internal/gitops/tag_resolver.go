package gitops

import (
	"context"
	"fmt"
	"strings"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/internal/registry"
	"github.com/kevinfinalboss/privateer/pkg/types"
	"github.com/kevinfinalboss/privateer/pkg/utils"
)

type TagResolver struct {
	logger          *logger.Logger
	config          *types.Config
	registryManager *registry.Manager
	clusterImages   map[string]*types.ImageInfo
}

func NewTagResolver(logger *logger.Logger, config *types.Config, registryManager *registry.Manager) *TagResolver {
	return &TagResolver{
		logger:          logger,
		config:          config,
		registryManager: registryManager,
		clusterImages:   make(map[string]*types.ImageInfo),
	}
}

func (tr *TagResolver) LoadClusterImages(clusterImages []*types.ImageInfo) {
	tr.logger.Info("loading_cluster_images_for_tag_resolution").
		Int("cluster_images", len(clusterImages)).
		Send()

	for _, img := range clusterImages {
		parsed := utils.ParseImageName(img.Image)

		keys := []string{
			parsed.Repository,
			fmt.Sprintf("%s/%s", parsed.Namespace, parsed.Repository),
			parsed.FullRepository,
		}

		for _, key := range keys {
			if _, exists := tr.clusterImages[key]; !exists {
				tr.clusterImages[key] = img
				tr.logger.Debug("cluster_image_indexed").
					Str("key", key).
					Str("image", img.Image).
					Str("namespace", img.Namespace).
					Send()
			}
		}
	}

	tr.logger.Info("cluster_images_indexed").
		Int("total_keys", len(tr.clusterImages)).
		Send()
}

func (tr *TagResolver) ResolveEmptyTag(ctx context.Context, registry, repository, tag string) (*types.TagResolutionResult, error) {
	result := &types.TagResolutionResult{
		OriginalTag: tag,
		WasResolved: false,
		Confidence:  0.0,
	}

	if !tr.isEmptyTag(tag) {
		result.ResolvedTag = tag
		result.Source = "original"
		result.Confidence = 1.0
		return result, nil
	}

	tr.logger.Info("resolving_empty_tag").
		Str("registry", registry).
		Str("repository", repository).
		Str("original_tag", tag).
		Send()

	clusterTag, clusterImage := tr.findTagInCluster(repository)
	if clusterTag != "" {
		tr.logger.Info("tag_found_in_cluster").
			Str("repository", repository).
			Str("cluster_tag", clusterTag).
			Str("cluster_image", clusterImage).
			Send()

		privateImage, exists := tr.validateInPrivateRegistry(ctx, clusterImage)
		if exists {
			result.ResolvedTag = clusterTag
			result.Source = "cluster"
			result.Confidence = 0.95
			result.WasResolved = true
			result.PrivateImage = privateImage
			result.ShouldReplace = tr.config.GitOps.TagResolution.AutoFillEmptyTags

			tr.logger.Info("cluster_tag_validated_in_private_registry").
				Str("cluster_image", clusterImage).
				Str("private_image", privateImage).
				Bool("should_replace", result.ShouldReplace).
				Send()

			return result, nil
		}
	}

	commonTags := tr.config.GitOps.TagResolution.CommonTagsToTry
	if len(commonTags) == 0 {
		commonTags = []string{"latest", "stable", "main", "v1"}
	}

	for _, commonTag := range commonTags {
		testImage := tr.buildImageName(registry, repository, commonTag)
		privateImage, exists := tr.validateInPrivateRegistry(ctx, testImage)
		if exists {
			result.ResolvedTag = commonTag
			result.Source = "registry_common"
			result.Confidence = 0.7
			result.WasResolved = true
			result.PrivateImage = privateImage
			result.ShouldReplace = tr.config.GitOps.TagResolution.AutoFillEmptyTags

			tr.logger.Info("common_tag_found_in_private_registry").
				Str("common_tag", commonTag).
				Str("test_image", testImage).
				Str("private_image", privateImage).
				Send()

			return result, nil
		}
	}

	fallbackTag := tr.config.GitOps.TagResolution.FallbackTag
	if fallbackTag == "" {
		fallbackTag = "latest"
	}

	result.ResolvedTag = fallbackTag
	result.Source = "default"
	result.Confidence = 0.3
	result.WasResolved = true
	result.ShouldReplace = false

	tr.logger.Warn("tag_resolution_fallback").
		Str("repository", repository).
		Str("fallback_tag", fallbackTag).
		Send()

	return result, nil
}

func (tr *TagResolver) findTagInCluster(repository string) (string, string) {
	searchKeys := []string{
		repository,
		strings.Split(repository, "/")[len(strings.Split(repository, "/"))-1],
	}

	if strings.Contains(repository, "/") {
		parts := strings.Split(repository, "/")
		searchKeys = append(searchKeys, parts[len(parts)-1])
	}

	for _, key := range searchKeys {
		if clusterImage, exists := tr.clusterImages[key]; exists {
			parsed := utils.ParseImageName(clusterImage.Image)

			tr.logger.Debug("cluster_image_match_found").
				Str("search_key", key).
				Str("cluster_image", clusterImage.Image).
				Str("extracted_tag", parsed.Tag).
				Send()

			return parsed.Tag, clusterImage.Image
		}
	}

	for key, clusterImage := range tr.clusterImages {
		if strings.Contains(key, repository) || strings.Contains(repository, key) {
			parsed := utils.ParseImageName(clusterImage.Image)

			tr.logger.Debug("cluster_image_partial_match").
				Str("search_repo", repository).
				Str("found_key", key).
				Str("cluster_image", clusterImage.Image).
				Str("extracted_tag", parsed.Tag).
				Send()

			return parsed.Tag, clusterImage.Image
		}
	}

	return "", ""
}

func (tr *TagResolver) validateInPrivateRegistry(ctx context.Context, publicImage string) (string, bool) {
	if tr.registryManager == nil {
		return "", false
	}

	imageInfo := &types.ImageInfo{
		Image: publicImage,
	}

	validatedMap, err := tr.registryManager.ValidateImagesBatch(ctx, []*types.ImageInfo{imageInfo}, tr.config)
	if err != nil {
		tr.logger.Warn("private_registry_validation_failed").
			Str("image", publicImage).
			Err(err).
			Send()
		return "", false
	}

	if privateImage, exists := validatedMap[publicImage]; exists {
		return privateImage, true
	}

	return "", false
}

func (tr *TagResolver) buildImageName(registry, repository, tag string) string {
	if registry == "" || registry == "docker.io" {
		return fmt.Sprintf("%s:%s", repository, tag)
	}
	return fmt.Sprintf("%s/%s:%s", registry, repository, tag)
}

func (tr *TagResolver) isEmptyTag(tag string) bool {
	emptyIndicators := []string{
		"",
		"\"\"",
		"''",
		"null",
		"undefined",
	}

	if tr.config.GitOps.TagResolution.ConsiderLatestEmpty && tag == "latest" {
		emptyIndicators = append(emptyIndicators, "latest")
	}

	for _, indicator := range emptyIndicators {
		if tag == indicator {
			return true
		}
	}

	if strings.Contains(tag, "{{") || strings.Contains(tag, "${") {
		return true
	}

	return false
}

func (tr *TagResolver) ProcessDetections(ctx context.Context, detections []types.ImageDetectionResult) ([]types.ImageDetectionResult, error) {
	if !tr.config.GitOps.TagResolution.Enabled {
		return detections, nil
	}

	var processedDetections []types.ImageDetectionResult

	for _, detection := range detections {
		processed := detection

		parsed := utils.ParseImageName(detection.FullImage)

		if tr.isEmptyTag(parsed.Tag) {
			tr.logger.Debug("processing_empty_tag_detection").
				Str("original_image", detection.FullImage).
				Str("repository", parsed.Repository).
				Str("tag", parsed.Tag).
				Send()

			result, err := tr.ResolveEmptyTag(ctx, parsed.Registry, parsed.Repository, parsed.Tag)
			if err != nil {
				tr.logger.Warn("tag_resolution_failed").
					Str("image", detection.FullImage).
					Err(err).
					Send()
				processedDetections = append(processedDetections, detection)
				continue
			}

			if result.WasResolved && result.ShouldReplace {
				newImage := tr.buildImageName(parsed.Registry, parsed.Repository, result.ResolvedTag)

				processed.FullImage = newImage
				processed.Tag = result.ResolvedTag
				processed.Confidence = result.Confidence

				processed.Context += fmt.Sprintf(" [tag resolvida: %s→%s via %s]",
					result.OriginalTag, result.ResolvedTag, result.Source)

				tr.logger.Info("tag_resolved_and_updated").
					Str("original", detection.FullImage).
					Str("resolved", newImage).
					Str("source", result.Source).
					Float64("confidence", result.Confidence).
					Send()
			}
		}

		processedDetections = append(processedDetections, processed)
	}

	return processedDetections, nil
}
