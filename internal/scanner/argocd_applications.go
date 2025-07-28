package scanner

import (
	"regexp"
	"strings"

	"github.com/kevinfinalboss/privateer/pkg/types"
)

func (fs *FileScanner) scanArgoCDApplication(content, filePath string, publicImageMap map[string]*types.ImageInfo) []types.ImageDetectionResult {
	var detections []types.ImageDetectionResult

	fs.logger.Debug("scanning_argocd_application").
		Str("file", filePath).
		Int("content_length", len(content)).
		Int("public_images", len(publicImageMap)).
		Send()

	detections = append(detections, fs.scanGenericYAML(content, filePath, publicImageMap)...)

	detections = append(detections, fs.scanArgoCDHelmValues(content, filePath, publicImageMap)...)

	if matches := imagePatterns["argocd_values"].FindAllStringSubmatch(content, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) > 1 {
				imageName := match[1]
				if _, isPublic := publicImageMap[imageName]; isPublic {
					detections = append(detections, types.ImageDetectionResult{
						Image:      imageName,
						Repository: fs.extractRepository(imageName),
						Tag:        fs.extractTag(imageName),
						Registry:   fs.extractRegistry(imageName),
						FullImage:  imageName,
						IsPublic:   true,
						LineNumber: fs.findLineNumber(content, imageName),
						Context:    "ArgoCD values block",
						Confidence: 0.8,
						FilePath:   filePath,
					})
				}
			}
		}
	}

	fs.logger.Debug("argocd_scan_completed").
		Str("file", filePath).
		Int("detections", len(detections)).
		Send()

	return detections
}

func (fs *FileScanner) scanArgoCDHelmValues(content, filePath string, publicImageMap map[string]*types.ImageInfo) []types.ImageDetectionResult {
	var detections []types.ImageDetectionResult

	lines := strings.Split(content, "\n")
	inValuesBlock := false
	valuesStartLine := 0
	valuesIndentLevel := 0
	valuesContent := strings.Builder{}

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if strings.Contains(line, "values:") && strings.Contains(line, "|") {
			inValuesBlock = true
			valuesStartLine = i + 1
			valuesIndentLevel = len(line) - len(strings.TrimLeft(line, " \t"))

			fs.logger.Debug("argocd_values_block_start").
				Str("file", filePath).
				Int("start_line", valuesStartLine).
				Int("indent_level", valuesIndentLevel).
				Send()
			continue
		}

		if inValuesBlock {
			currentIndentLevel := len(line) - len(strings.TrimLeft(line, " \t"))

			if trimmedLine == "" {
				valuesContent.WriteString(line + "\n")
				continue
			}

			if currentIndentLevel <= valuesIndentLevel && trimmedLine != "" {
				inValuesBlock = false
				break
			}

			valuesContent.WriteString(line + "\n")
		}
	}

	if valuesContent.Len() > 0 {
		valuesText := valuesContent.String()

		fs.logger.Debug("argocd_values_content_extracted").
			Str("file", filePath).
			Int("content_length", len(valuesText)).
			Int("values_start_line", valuesStartLine).
			Send()

		detections = append(detections, fs.scanArgoCDImageField(valuesText, filePath, publicImageMap, valuesStartLine)...)
		detections = append(detections, fs.scanArgoCDHelmSeparatedFields(valuesText, filePath, publicImageMap, valuesStartLine)...)
		detections = append(detections, fs.scanArgoCDInitContainers(valuesText, filePath, publicImageMap, valuesStartLine)...)
	}

	return detections
}

func (fs *FileScanner) scanArgoCDImageField(valuesContent, filePath string, publicImageMap map[string]*types.ImageInfo, baseLineOffset int) []types.ImageDetectionResult {
	var detections []types.ImageDetectionResult
	lines := strings.Split(valuesContent, "\n")

	imageRegex := regexp.MustCompile(`^\s*image:\s*["']?([^"'\s\n]+)["']?\s*$`)

	for lineNum, line := range lines {
		if matches := imageRegex.FindStringSubmatch(line); len(matches) > 1 {
			imageName := strings.Trim(matches[1], `"' `)

			if _, isPublic := publicImageMap[imageName]; isPublic {
				detections = append(detections, types.ImageDetectionResult{
					Image:      imageName,
					Repository: fs.extractRepository(imageName),
					Tag:        fs.extractTag(imageName),
					Registry:   fs.extractRegistry(imageName),
					FullImage:  imageName,
					IsPublic:   true,
					LineNumber: baseLineOffset + lineNum,
					Context:    strings.TrimSpace(line),
					Confidence: 0.9,
					FilePath:   filePath,
				})

				fs.logger.Debug("argocd_direct_image_detected").
					Str("file", filePath).
					Str("image", imageName).
					Int("line", baseLineOffset+lineNum).
					Send()
			}
		}
	}

	return detections
}

func (fs *FileScanner) scanArgoCDHelmSeparatedFields(valuesContent, filePath string, publicImageMap map[string]*types.ImageInfo, baseLineOffset int) []types.ImageDetectionResult {
	var detections []types.ImageDetectionResult
	lines := strings.Split(valuesContent, "\n")

	var currentRegistry string
	var currentRepository string
	var currentTag string
	var repoLine int
	inImageSection := false
	imageSectionIndent := 0

	repositoryRegex := regexp.MustCompile(`^\s*repository:\s*["']?([^"'\s\n]+)["']?\s*$`)
	tagRegex := regexp.MustCompile(`^\s*tag:\s*["']?([^"'\s\n]+)["']?\s*$`)
	registryRegex := regexp.MustCompile(`^\s*registry:\s*["']?([^"'\s\n]+)["']?\s*$`)

	for lineNum, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))

		if trimmedLine == "image:" {
			inImageSection = true
			imageSectionIndent = currentIndent
			currentRegistry = ""
			currentRepository = ""
			currentTag = ""

			fs.logger.Debug("argocd_helm_image_section_start").
				Str("file", filePath).
				Int("line", baseLineOffset+lineNum).
				Int("indent", imageSectionIndent).
				Send()
			continue
		}

		if inImageSection {
			if trimmedLine == "" {
				continue
			}

			if currentIndent <= imageSectionIndent && trimmedLine != "" {
				if currentRepository != "" && currentTag != "" {
					detection := fs.buildHelmImageDetection(currentRegistry, currentRepository, currentTag, repoLine, filePath, publicImageMap)
					if detection != nil {
						detections = append(detections, *detection)
					}
				}
				inImageSection = false
				imageSectionIndent = 0
				currentRegistry = ""
				currentRepository = ""
				currentTag = ""
			}

			if matches := registryRegex.FindStringSubmatch(line); len(matches) > 1 {
				currentRegistry = strings.Trim(matches[1], `"' `)
				fs.logger.Debug("argocd_helm_registry_found").
					Str("registry", currentRegistry).
					Int("line", baseLineOffset+lineNum).
					Send()
			}

			if matches := repositoryRegex.FindStringSubmatch(line); len(matches) > 1 {
				currentRepository = strings.Trim(matches[1], `"' `)
				repoLine = baseLineOffset + lineNum
				fs.logger.Debug("argocd_helm_repository_found").
					Str("repository", currentRepository).
					Int("line", repoLine).
					Send()
			}

			if matches := tagRegex.FindStringSubmatch(line); len(matches) > 1 {
				currentTag = strings.Trim(matches[1], `"' `)
				fs.logger.Debug("argocd_helm_tag_found").
					Str("tag", currentTag).
					Int("line", baseLineOffset+lineNum).
					Send()
			}

			if inImageSection && currentRepository != "" && currentTag != "" {
				detection := fs.buildHelmImageDetection(currentRegistry, currentRepository, currentTag, repoLine, filePath, publicImageMap)
				if detection != nil {
					detections = append(detections, *detection)
					fs.logger.Info("argocd_helm_separated_image_detected").
						Str("file", filePath).
						Str("full_image", detection.FullImage).
						Int("line", detection.LineNumber).
						Send()
				}

				inImageSection = false
				currentRegistry = ""
				currentRepository = ""
				currentTag = ""
			}
		}
	}

	if inImageSection && currentRepository != "" && currentTag != "" {
		detection := fs.buildHelmImageDetection(currentRegistry, currentRepository, currentTag, repoLine, filePath, publicImageMap)
		if detection != nil {
			detections = append(detections, *detection)
		}
	}

	return detections
}

func (fs *FileScanner) buildHelmImageDetection(registry, repository, tag string, lineNumber int, filePath string, publicImageMap map[string]*types.ImageInfo) *types.ImageDetectionResult {
	var fullImage string
	var detectedRegistry string

	if registry != "" {
		detectedRegistry = registry
		fullImage = registry + "/" + repository + ":" + tag
	} else if fs.repositoryContainsRegistry(repository) {
		detectedRegistry = fs.extractRegistryFromRepository(repository)
		repositoryPart := fs.extractRepositoryFromCombined(repository)
		fullImage = repository + ":" + tag
		repository = repositoryPart
	} else {
		detectedRegistry = "docker.io"
		fullImage = repository + ":" + tag

		dockerIOFormat := "docker.io/" + repository + ":" + tag
		if _, exists := publicImageMap[dockerIOFormat]; exists {
			fullImage = dockerIOFormat
		}
	}

	fs.logger.Debug("checking_image_in_public_map").
		Str("full_image", fullImage).
		Str("registry", detectedRegistry).
		Str("repository", repository).
		Str("tag", tag).
		Bool("exists", publicImageMap[fullImage] != nil).
		Send()

	if _, isPublic := publicImageMap[fullImage]; isPublic {
		return &types.ImageDetectionResult{
			Image:      fullImage,
			Repository: repository,
			Tag:        tag,
			Registry:   detectedRegistry,
			FullImage:  fullImage,
			IsPublic:   true,
			LineNumber: lineNumber,
			Context:    fs.buildHelmContext(registry, repository, tag, "argocd_helm"),
			Confidence: 0.95,
			FilePath:   filePath,
		}
	}

	return nil
}

func (fs *FileScanner) scanArgoCDInitContainers(valuesContent, filePath string, publicImageMap map[string]*types.ImageInfo, baseLineOffset int) []types.ImageDetectionResult {
	var detections []types.ImageDetectionResult
	lines := strings.Split(valuesContent, "\n")

	initContainerImageRegex := regexp.MustCompile(`^\s*-?\s*image:\s*["']?([^"'\s\n]+)["']?\s*$`)
	inInitContainers := false
	initContainersIndent := 0

	for lineNum, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))

		if strings.Contains(trimmedLine, "initContainers:") {
			inInitContainers = true
			initContainersIndent = currentIndent
			fs.logger.Debug("argocd_init_containers_section_start").
				Str("file", filePath).
				Int("line", baseLineOffset+lineNum).
				Int("indent", initContainersIndent).
				Send()
			continue
		}

		if inInitContainers {
			if trimmedLine == "" {
				continue
			}

			if currentIndent <= initContainersIndent && trimmedLine != "" && !strings.HasPrefix(trimmedLine, "-") {
				inInitContainers = false
				continue
			}

			if matches := initContainerImageRegex.FindStringSubmatch(line); len(matches) > 1 {
				imageName := strings.Trim(matches[1], `"' `)

				if _, isPublic := publicImageMap[imageName]; isPublic {
					detections = append(detections, types.ImageDetectionResult{
						Image:      imageName,
						Repository: fs.extractRepository(imageName),
						Tag:        fs.extractTag(imageName),
						Registry:   fs.extractRegistry(imageName),
						FullImage:  imageName,
						IsPublic:   true,
						LineNumber: baseLineOffset + lineNum,
						Context:    strings.TrimSpace(line),
						Confidence: 0.9,
						FilePath:   filePath,
					})

					fs.logger.Debug("argocd_init_container_image_detected").
						Str("file", filePath).
						Str("image", imageName).
						Int("line", baseLineOffset+lineNum).
						Send()
				}
			}
		}
	}

	return detections
}
