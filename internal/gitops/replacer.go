package gitops

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
	"github.com/kevinfinalboss/privateer/pkg/utils"
)

type ImageReplacer struct {
	logger *logger.Logger
	config *types.Config
}

func NewImageReplacer(logger *logger.Logger, config *types.Config) *ImageReplacer {
	return &ImageReplacer{
		logger: logger,
		config: config,
	}
}

func (ir *ImageReplacer) ReplaceImagesInContent(content string, replacements []types.ImageReplacement) (string, []types.ImageReplacement, error) {
	ir.logger.Debug("replacing_images_in_content").
		Int("replacements", len(replacements)).
		Send()

	modifiedContent := content
	var actualReplacements []types.ImageReplacement

	for _, replacement := range replacements {
		newContent, wasReplaced, err := ir.replaceImageInContent(modifiedContent, replacement)
		if err != nil {
			return content, nil, fmt.Errorf("falha ao substituir imagem %s: %w", replacement.SourceImage, err)
		}

		if wasReplaced {
			modifiedContent = newContent
			actualReplacements = append(actualReplacements, replacement)

			ir.logger.Debug("image_replaced").
				Str("source", replacement.SourceImage).
				Str("target", replacement.TargetImage).
				Str("type", replacement.FileType).
				Send()
		}
	}

	if err := ir.validateReplacedContent(modifiedContent); err != nil {
		return content, nil, fmt.Errorf("validação do conteúdo falhou: %w", err)
	}

	ir.logger.Debug("content_replacement_completed").
		Int("actual_replacements", len(actualReplacements)).
		Send()

	return modifiedContent, actualReplacements, nil
}

func (ir *ImageReplacer) replaceImageInContent(content string, replacement types.ImageReplacement) (string, bool, error) {
	switch replacement.FileType {
	case "helm_separated":
		return ir.replaceHelmSeparatedPrecise(content, replacement)
	case "helm_combined":
		return ir.replaceHelmCombined(content, replacement)
	case "kustomize":
		return ir.replaceKustomize(content, replacement)
	case "kubernetes_manifest":
		return ir.replaceKubernetesManifest(content, replacement)
	default:
		return ir.replaceGeneric(content, replacement)
	}
}

func (ir *ImageReplacer) replaceKubernetesManifest(content string, replacement types.ImageReplacement) (string, bool, error) {
	sourceRepo := ir.extractRepository(replacement.SourceImage)
	sourceTag := ir.extractTag(replacement.SourceImage)
	targetTag := ir.extractTag(replacement.TargetImage)

	patterns := []string{
		fmt.Sprintf(`(\s+image:\s*["']?)%s(["']?\s*)`, regexp.QuoteMeta(replacement.SourceImage)),
		fmt.Sprintf(`(\s+image:\s*["']?)%s:%s(["']?\s*)`, regexp.QuoteMeta(sourceRepo), regexp.QuoteMeta(sourceTag)),
	}

	targetImage := replacement.TargetImage
	if !strings.Contains(targetImage, ":") {
		targetImage += ":" + targetTag
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(content) {
			newContent := re.ReplaceAllString(content, "${1}"+targetImage+"${2}")
			if newContent != content {
				return newContent, true, nil
			}
		}
	}

	return content, false, nil
}

func (ir *ImageReplacer) replaceHelmSeparatedPrecise(content string, replacement types.ImageReplacement) (string, bool, error) {
	sourceRegistry := utils.ExtractRegistry(replacement.SourceImage)
	sourceRepo := ir.extractSourceRepository(replacement.SourceImage)
	sourceTag := utils.ExtractTag(replacement.SourceImage)

	targetRegistry := utils.ExtractRegistry(replacement.TargetImage)
	targetRepo := ir.extractTargetRepository(replacement.TargetImage)
	targetTag := utils.ExtractTag(replacement.TargetImage)

	ir.logger.Debug("helm_separated_precise_replacement").
		Str("source_registry", sourceRegistry).
		Str("source_repo", sourceRepo).
		Str("source_tag", sourceTag).
		Str("target_registry", targetRegistry).
		Str("target_repo", targetRepo).
		Str("target_tag", targetTag).
		Str("line_number", fmt.Sprintf("%d", replacement.LineNumber)).
		Send()

	lines := strings.Split(content, "\n")
	modified := false

	imageSection := ir.findImageSectionForLine(lines, replacement.LineNumber, sourceRegistry, sourceRepo, sourceTag)
	if imageSection.found {
		ir.logger.Debug("image_section_found").
			Int("start_line", imageSection.startLine).
			Int("end_line", imageSection.endLine).
			Int("registry_line", imageSection.registryLine).
			Int("repo_line", imageSection.repoLine).
			Int("tag_line", imageSection.tagLine).
			Send()

		if imageSection.registryLine > 0 && imageSection.registryLine <= len(lines) {
			registryPattern := fmt.Sprintf(`(\s*registry:\s*["']?)%s(["']?\s*)`, regexp.QuoteMeta(sourceRegistry))
			re := regexp.MustCompile(registryPattern)
			if re.MatchString(lines[imageSection.registryLine-1]) {
				lines[imageSection.registryLine-1] = re.ReplaceAllString(lines[imageSection.registryLine-1], "${1}"+targetRegistry+"${2}")
				modified = true
				ir.logger.Info("helm_registry_replaced_precise").
					Str("old", sourceRegistry).
					Str("new", targetRegistry).
					Int("line", imageSection.registryLine).
					Send()
			}
		}

		if imageSection.repoLine > 0 && imageSection.repoLine <= len(lines) {
			repoPattern := fmt.Sprintf(`(\s*repository:\s*["']?)%s(["']?\s*)`, regexp.QuoteMeta(sourceRepo))
			re := regexp.MustCompile(repoPattern)
			if re.MatchString(lines[imageSection.repoLine-1]) {
				lines[imageSection.repoLine-1] = re.ReplaceAllString(lines[imageSection.repoLine-1], "${1}"+targetRepo+"${2}")
				modified = true
				ir.logger.Info("helm_repository_replaced_precise").
					Str("old", sourceRepo).
					Str("new", targetRepo).
					Int("line", imageSection.repoLine).
					Send()
			}
		}

		if imageSection.tagLine > 0 && imageSection.tagLine <= len(lines) && sourceTag != targetTag {
			tagPattern := fmt.Sprintf(`(\s*tag:\s*["']?)%s(["']?\s*)`, regexp.QuoteMeta(sourceTag))
			re := regexp.MustCompile(tagPattern)
			if re.MatchString(lines[imageSection.tagLine-1]) {
				lines[imageSection.tagLine-1] = re.ReplaceAllString(lines[imageSection.tagLine-1], "${1}"+targetTag+"${2}")
				modified = true
				ir.logger.Info("helm_tag_replaced_precise").
					Str("old", sourceTag).
					Str("new", targetTag).
					Int("line", imageSection.tagLine).
					Send()
			}
		}
	} else {
		ir.logger.Warn("image_section_not_found_fallback_to_generic").
			Int("line_number", replacement.LineNumber).
			Str("source_image", replacement.SourceImage).
			Send()
		return ir.replaceGeneric(content, replacement)
	}

	if modified {
		ir.logger.Info("helm_separated_precise_replacement_completed").
			Bool("modified", true).
			Send()
		return strings.Join(lines, "\n"), true, nil
	}

	return content, false, nil
}

func (ir *ImageReplacer) replaceHelmCombined(content string, replacement types.ImageReplacement) (string, bool, error) {
	sourceImage := replacement.SourceImage
	targetImage := replacement.TargetImage

	sourceParsed := utils.ParseImageName(sourceImage)
	targetParsed := utils.ParseImageName(targetImage)

	ir.logger.Debug("helm_combined_replacement").
		Str("source_image", sourceImage).
		Str("target_image", targetImage).
		Str("source_combined_repo", fmt.Sprintf("%s/%s", sourceParsed.Registry, sourceParsed.FullRepository)).
		Str("target_combined_repo", fmt.Sprintf("%s/%s", targetParsed.Registry, targetParsed.FullRepository)).
		Send()

	lines := strings.Split(content, "\n")
	modified := false

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if strings.Contains(trimmedLine, "repository:") {
			sourceCombinedRepo := fmt.Sprintf("%s/%s", sourceParsed.Registry, sourceParsed.FullRepository)
			if sourceParsed.Registry == "docker.io" {
				sourceCombinedRepo = sourceParsed.FullRepository
			}

			repoPattern := fmt.Sprintf(`(\s*repository:\s*["']?)%s(["']?\s*)`, regexp.QuoteMeta(sourceCombinedRepo))
			re := regexp.MustCompile(repoPattern)
			if re.MatchString(line) {
				targetCombinedRepo := fmt.Sprintf("%s/%s", targetParsed.Registry, targetParsed.FullRepository)
				if targetParsed.Registry == "docker.io" {
					targetCombinedRepo = targetParsed.FullRepository
				}

				lines[i] = re.ReplaceAllString(line, "${1}"+targetCombinedRepo+"${2}")
				modified = true
				ir.logger.Info("helm_combined_repository_replaced").
					Str("old", sourceCombinedRepo).
					Str("new", targetCombinedRepo).
					Int("line", i+1).
					Send()
			}
		}

		if strings.Contains(trimmedLine, "tag:") && sourceParsed.Tag != targetParsed.Tag {
			tagPattern := fmt.Sprintf(`(\s*tag:\s*["']?)%s(["']?\s*)`, regexp.QuoteMeta(sourceParsed.Tag))
			re := regexp.MustCompile(tagPattern)
			if re.MatchString(line) {
				lines[i] = re.ReplaceAllString(line, "${1}"+targetParsed.Tag+"${2}")
				modified = true
				ir.logger.Info("helm_combined_tag_replaced").
					Str("old", sourceParsed.Tag).
					Str("new", targetParsed.Tag).
					Int("line", i+1).
					Send()
			}
		}
	}

	if modified {
		ir.logger.Info("helm_combined_replacement_completed").
			Bool("modified", true).
			Send()
		return strings.Join(lines, "\n"), true, nil
	}

	return content, false, nil
}

type ImageSection struct {
	found        bool
	startLine    int
	endLine      int
	registryLine int
	repoLine     int
	tagLine      int
}

func (ir *ImageReplacer) findImageSectionForLine(lines []string, targetLine int, expectedRegistry, expectedRepo, expectedTag string) ImageSection {
	section := ImageSection{found: false}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "image:") {
			section = ImageSection{
				found:     false,
				startLine: i + 1,
			}

			var currentRegistry, currentRepo, currentTag string
			var registryLine, repoLine, tagLine int

			for j := i + 1; j < len(lines); j++ {
				nextLine := lines[j]
				nextTrimmed := strings.TrimSpace(nextLine)

				if !strings.HasPrefix(nextLine, " ") && !strings.HasPrefix(nextLine, "\t") &&
					nextTrimmed != "" && !strings.HasPrefix(nextTrimmed, "registry:") &&
					!strings.HasPrefix(nextTrimmed, "repository:") && !strings.HasPrefix(nextTrimmed, "tag:") {
					section.endLine = j
					break
				}

				if strings.Contains(nextTrimmed, "registry:") {
					if matches := regexp.MustCompile(`registry:\s*["']?([^"'\s]+)["']?`).FindStringSubmatch(nextLine); len(matches) > 1 {
						currentRegistry = strings.Trim(matches[1], `"'`)
						registryLine = j + 1
					}
				}

				if strings.Contains(nextTrimmed, "repository:") {
					if matches := regexp.MustCompile(`repository:\s*["']?([^"'\s]+)["']?`).FindStringSubmatch(nextLine); len(matches) > 1 {
						currentRepo = strings.Trim(matches[1], `"'`)
						repoLine = j + 1
					}
				}

				if strings.Contains(nextTrimmed, "tag:") {
					if matches := regexp.MustCompile(`tag:\s*["']?([^"'\s]+)["']?`).FindStringSubmatch(nextLine); len(matches) > 1 {
						currentTag = strings.Trim(matches[1], `"'`)
						tagLine = j + 1
					}
				}

				if j+1 == len(lines) {
					section.endLine = j + 1
					break
				}
			}

			if currentRegistry == expectedRegistry && currentRepo == expectedRepo && currentTag == expectedTag {
				if repoLine == targetLine || (targetLine >= section.startLine && targetLine <= section.endLine) {
					section.found = true
					section.registryLine = registryLine
					section.repoLine = repoLine
					section.tagLine = tagLine

					ir.logger.Debug("matching_image_section_found").
						Str("registry", currentRegistry).
						Str("repository", currentRepo).
						Str("tag", currentTag).
						Int("start", section.startLine).
						Int("end", section.endLine).
						Send()

					return section
				}
			}

			i = section.endLine - 1
		}
	}

	return ImageSection{found: false}
}

func (ir *ImageReplacer) extractSourceRepository(imageName string) string {
	parsed := utils.ParseImageName(imageName)
	if parsed.Registry == "docker.io" && parsed.Namespace != "" && parsed.Namespace != "library" {
		return fmt.Sprintf("%s/%s", parsed.Namespace, parsed.Repository)
	}
	return parsed.Repository
}

func (ir *ImageReplacer) extractTargetRepository(imageName string) string {
	parsed := utils.ParseImageName(imageName)

	if parsed.Namespace != "" && parsed.Namespace != "library" {
		return fmt.Sprintf("%s/%s", parsed.Namespace, parsed.Repository)
	}

	return parsed.Repository
}

func (ir *ImageReplacer) replaceKustomize(content string, replacement types.ImageReplacement) (string, bool, error) {
	sourceRepo := ir.extractRepository(replacement.SourceImage)
	sourceTag := ir.extractTag(replacement.SourceImage)
	targetRepo := ir.extractRepository(replacement.TargetImage)
	targetTag := ir.extractTag(replacement.TargetImage)

	lines := strings.Split(content, "\n")
	modified := false
	inImagesSection := false
	currentImageSection := false

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "images:") {
			inImagesSection = true
			continue
		}

		if inImagesSection {
			if strings.HasPrefix(trimmedLine, "- name:") || strings.HasPrefix(trimmedLine, "-name:") {
				namePattern := fmt.Sprintf(`(-\s*name:\s*["']?)%s(["']?\s*)`, regexp.QuoteMeta(sourceRepo))
				re := regexp.MustCompile(namePattern)
				if re.MatchString(line) {
					currentImageSection = true
				} else {
					currentImageSection = false
				}
			}

			if currentImageSection {
				if strings.Contains(trimmedLine, "newName:") {
					newNamePattern := fmt.Sprintf(`(\s*newName:\s*["']?)%s(["']?\s*)`, regexp.QuoteMeta(sourceRepo))
					re := regexp.MustCompile(newNamePattern)
					if re.MatchString(line) {
						lines[i] = re.ReplaceAllString(line, "${1}"+targetRepo+"${2}")
						modified = true
					}
				}

				if strings.Contains(trimmedLine, "newTag:") {
					newTagPattern := fmt.Sprintf(`(\s*newTag:\s*["']?)%s(["']?\s*)`, regexp.QuoteMeta(sourceTag))
					re := regexp.MustCompile(newTagPattern)
					if re.MatchString(line) {
						lines[i] = re.ReplaceAllString(line, "${1}"+targetTag+"${2}")
						modified = true
					}
				}
			}

			if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") &&
				trimmedLine != "" && !strings.HasPrefix(trimmedLine, "-") {
				inImagesSection = false
				currentImageSection = false
			}
		}
	}

	if modified {
		return strings.Join(lines, "\n"), true, nil
	}

	return content, false, nil
}

func (ir *ImageReplacer) replaceGeneric(content string, replacement types.ImageReplacement) (string, bool, error) {
	patterns := []string{
		fmt.Sprintf(`(\s+image:\s*["']?)%s(["']?\s*)`, regexp.QuoteMeta(replacement.SourceImage)),
		fmt.Sprintf(`(:\s*["']?)%s(["']?)`, regexp.QuoteMeta(replacement.SourceImage)),
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(content) {
			newContent := re.ReplaceAllString(content, "${1}"+replacement.TargetImage+"${2}")
			if newContent != content {
				return newContent, true, nil
			}
		}
	}

	return content, false, nil
}

func (ir *ImageReplacer) validateReplacedContent(content string) error {
	if !ir.config.GitOps.ValidationRules.ValidateYAML {
		return nil
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		if strings.Contains(line, ":") {
			parts := strings.Split(line, ":")
			if len(parts) < 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			if key == "" {
				return fmt.Errorf("chave vazia na linha %d: %s", i+1, line)
			}
		}
	}

	if ir.config.GitOps.ValidationRules.ValidateBrackets {
		if strings.Count(content, "{") != strings.Count(content, "}") {
			return fmt.Errorf("chaves desbalanceadas no conteúdo")
		}

		if strings.Count(content, "[") != strings.Count(content, "]") {
			return fmt.Errorf("colchetes desbalanceados no conteúdo")
		}
	}

	return nil
}

func (ir *ImageReplacer) extractRepository(imageName string) string {
	return utils.ExtractRepository(imageName)
}

func (ir *ImageReplacer) extractTag(imageName string) string {
	return utils.ExtractTag(imageName)
}

func (ir *ImageReplacer) PreviewReplacements(content string, replacements []types.ImageReplacement) ([]string, error) {
	var previews []string

	for _, replacement := range replacements {
		lines := strings.Split(content, "\n")

		for i, line := range lines {
			if strings.Contains(line, replacement.SourceImage) {
				preview := fmt.Sprintf("Linha %d: %s → %s",
					i+1,
					strings.TrimSpace(line),
					strings.ReplaceAll(strings.TrimSpace(line), replacement.SourceImage, replacement.TargetImage))
				previews = append(previews, preview)
			}
		}
	}

	return previews, nil
}

func (ir *ImageReplacer) GetReplacementStats(replacements []types.ImageReplacement) map[string]int {
	stats := make(map[string]int)

	for _, replacement := range replacements {
		stats["total"]++
		stats[replacement.FileType]++

		if strings.Contains(replacement.TargetImage, "docker.io") {
			stats["docker_hub"]++
		} else if strings.Contains(replacement.TargetImage, "ecr.") {
			stats["ecr"]++
		} else if strings.Contains(replacement.TargetImage, "harbor") {
			stats["harbor"]++
		} else {
			stats["other_registry"]++
		}
	}

	return stats
}
