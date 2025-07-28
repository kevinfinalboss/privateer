package scanner

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kevinfinalboss/privateer/pkg/types"
	"github.com/kevinfinalboss/privateer/pkg/utils"
)

func (fs *FileScanner) scanHelmValues(content, filePath string, publicImageMap map[string]*types.ImageInfo) []types.ImageDetectionResult {
	var detections []types.ImageDetectionResult
	lines := strings.Split(content, "\n")

	fs.logger.Debug("scanning_helm_values").
		Str("file", filePath).
		Int("lines", len(lines)).
		Send()

	var currentRegistry string
	var currentRepository string
	var currentTag string
	var registryLine, repoLine, tagLine int
	inImageSection := false

	for lineNum, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "image:") {
			inImageSection = true
			currentRegistry = ""
			currentRepository = ""
			currentTag = ""
			continue
		}

		if !inImageSection {
			continue
		}

		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") &&
			trimmedLine != "" && !strings.HasPrefix(trimmedLine, "registry:") &&
			!strings.HasPrefix(trimmedLine, "repository:") && !strings.HasPrefix(trimmedLine, "tag:") {
			inImageSection = false
			continue
		}

		if matches := regexp.MustCompile(`registry:\s*["']?([^"'\s]+)["']?`).FindStringSubmatch(line); len(matches) > 1 {
			currentRegistry = strings.Trim(matches[1], `"'`)
			registryLine = lineNum + 1
			fs.logger.Debug("helm_registry_detected").
				Str("registry", currentRegistry).
				Int("line", registryLine).
				Send()
		}

		if matches := regexp.MustCompile(`repository:\s*["']?([^"'\s]+)["']?`).FindStringSubmatch(line); len(matches) > 1 {
			currentRepository = strings.Trim(matches[1], `"'`)
			repoLine = lineNum + 1
			fs.logger.Debug("helm_repository_detected").
				Str("repository", currentRepository).
				Int("line", repoLine).
				Send()
		}

		if matches := regexp.MustCompile(`tag:\s*["']?([^"'\s]+)["']?`).FindStringSubmatch(line); len(matches) > 1 {
			currentTag = strings.Trim(matches[1], `"'`)
			tagLine = lineNum + 1
			fs.logger.Debug("helm_tag_detected").
				Str("tag", currentTag).
				Int("line", tagLine).
				Send()
		}

		if (currentRegistry != "" && currentRepository != "" && currentTag != "") ||
			(currentRepository != "" && currentTag != "" && fs.repositoryContainsRegistry(currentRepository)) {

			var detectedRegistry, detectedRepository, fullImage string
			var fileType string

			if currentRegistry != "" {
				detectedRegistry = currentRegistry
				detectedRepository = currentRepository
				fileType = "helm_separated"
			} else {
				detectedRegistry = fs.extractRegistryFromRepository(currentRepository)
				detectedRepository = fs.extractRepositoryFromCombined(currentRepository)
				fileType = "helm_combined"
			}

			fs.logger.Debug("helm_complete_image_found").
				Str("detected_registry", detectedRegistry).
				Str("detected_repository", detectedRepository).
				Str("tag", currentTag).
				Str("file_type", fileType).
				Send()

			if utils.IsPublicRegistry(detectedRegistry) {
				if detectedRegistry == "docker.io" {
					fullImage = utils.BuildDockerIOImageName(detectedRepository, currentTag)
				} else {
					fullImage = utils.BuildFullImageName(detectedRegistry, detectedRepository, currentTag)
				}

				fs.logger.Debug("checking_public_image").
					Str("full_image", fullImage).
					Bool("is_public_registry", true).
					Send()

				if _, isInCluster := publicImageMap[fullImage]; isInCluster {
					detection := types.ImageDetectionResult{
						Image:      fullImage,
						Repository: utils.ExtractRepository(fullImage),
						Tag:        currentTag,
						Registry:   detectedRegistry,
						FullImage:  fullImage,
						IsPublic:   true,
						LineNumber: repoLine,
						Context:    fs.buildHelmContext(currentRegistry, currentRepository, currentTag, fileType),
						Confidence: 0.95,
						FilePath:   filePath,
					}

					detections = append(detections, detection)

					fs.logger.Info("helm_image_detected").
						Str("file", filePath).
						Str("type", fileType).
						Str("registry", detectedRegistry).
						Str("repository", detectedRepository).
						Str("tag", currentTag).
						Str("full_image", fullImage).
						Int("registry_line", registryLine).
						Int("repo_line", repoLine).
						Int("tag_line", tagLine).
						Send()
				} else {
					fs.logger.Debug("public_image_not_in_cluster").
						Str("full_image", fullImage).
						Send()
				}
			} else {
				fs.logger.Debug("private_registry_detected").
					Str("registry", detectedRegistry).
					Send()
			}

			currentRegistry = ""
			currentRepository = ""
			currentTag = ""
		}
	}

	inlineDetections := fs.scanGenericYAML(content, filePath, publicImageMap)
	detections = append(detections, inlineDetections...)

	return detections
}

func (fs *FileScanner) repositoryContainsRegistry(repository string) bool {
	parts := strings.Split(repository, "/")
	if len(parts) >= 2 && strings.Contains(parts[0], ".") {
		return true
	}
	return false
}

func (fs *FileScanner) extractRegistryFromRepository(repository string) string {
	if fs.repositoryContainsRegistry(repository) {
		return strings.Split(repository, "/")[0]
	}
	return "docker.io"
}

func (fs *FileScanner) extractRepositoryFromCombined(repository string) string {
	if fs.repositoryContainsRegistry(repository) {
		parts := strings.Split(repository, "/")
		return strings.Join(parts[1:], "/")
	}
	return repository
}

func (fs *FileScanner) buildHelmContext(registry, repository, tag, fileType string) string {
	if fileType == "helm_separated" {
		return fmt.Sprintf("registry: %s, repository: %s, tag: %s", registry, repository, tag)
	}
	return fmt.Sprintf("repository: %s, tag: %s (combined)", repository, tag)
}
