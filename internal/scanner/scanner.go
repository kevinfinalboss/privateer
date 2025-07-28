package scanner

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"github.com/kevinfinalboss/privateer/internal/github"
	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
	"github.com/kevinfinalboss/privateer/pkg/utils"
)

type FileScanner struct {
	githubClient *github.Client
	logger       *logger.Logger
	config       *types.Config
}

type FileType int

const (
	FileTypeUnknown FileType = iota
	FileTypeKubernetesManifest
	FileTypeHelmValues
	FileTypeArgoCDApplication
	FileTypeKustomization
	FileTypeDockerCompose
)

var (
	imagePatterns = map[string]*regexp.Regexp{
		"yaml_image":        regexp.MustCompile(`(?m)^\s*image:\s*["']?([^"'\s]+)["']?`),
		"helm_repository":   regexp.MustCompile(`(?m)^\s*repository:\s*["']?([^"'\s]+)["']?`),
		"helm_tag":          regexp.MustCompile(`(?m)^\s*tag:\s*["']?([^"'\s]+)["']?`),
		"kustomize_newName": regexp.MustCompile(`(?m)^\s*newName:\s*["']?([^"'\s]+)["']?`),
		"kustomize_newTag":  regexp.MustCompile(`(?m)^\s*newTag:\s*["']?([^"'\s]+)["']?`),
		"argocd_values":     regexp.MustCompile(`(?m)values:\s*\|[\s\S]*?image:\s*["']?([^"'\s]+)["']?`),
	}

	fileTypeIndicators = map[FileType][]string{
		FileTypeKubernetesManifest: {"apiVersion:", "kind:", "metadata:", "spec:"},
		FileTypeHelmValues:         {"# Default values", "image:", "repository:", "tag:"},
		FileTypeArgoCDApplication:  {"apiVersion: argoproj.io", "kind: Application", "spec:", "source:"},
		FileTypeKustomization:      {"apiVersion: kustomize", "kind: Kustomization", "resources:", "images:"},
		FileTypeDockerCompose:      {"version:", "services:", "image:"},
	}
)

func NewFileScanner(githubClient *github.Client, logger *logger.Logger, config *types.Config) *FileScanner {
	return &FileScanner{
		githubClient: githubClient,
		logger:       logger,
		config:       config,
	}
}

func (fs *FileScanner) ScanRepositoryForImages(ctx context.Context, repoConfig types.GitHubRepositoryConfig, publicImages []*types.ImageInfo) ([]types.ImageDetectionResult, error) {
	fs.logger.Info("scanning_repository_for_images").
		Str("repository", repoConfig.Name).
		Int("public_images", len(publicImages)).
		Send()

	owner, repo, err := fs.parseRepositoryName(repoConfig.Name)
	if err != nil {
		return nil, err
	}

	repoManager := github.NewRepositoryManager(fs.githubClient)
	files, err := repoManager.ListRepositoryFiles(ctx, repoConfig)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar arquivos do repositório: %w", err)
	}

	relevantFiles := repoManager.GetFilesByExtension(files, []string{"yaml", "yml"})

	var allDetections []types.ImageDetectionResult
	publicImageMap := fs.createPublicImageMap(publicImages)

	for _, file := range relevantFiles {
		detections, err := fs.scanFile(ctx, owner, repo, file.Path, publicImageMap)
		if err != nil {
			fs.logger.Warn("file_scan_failed").
				Str("file", file.Path).
				Err(err).
				Send()
			continue
		}

		allDetections = append(allDetections, detections...)
	}

	fs.logger.Info("repository_scan_completed").
		Str("repository", repoConfig.Name).
		Int("files_scanned", len(relevantFiles)).
		Int("images_detected", len(allDetections)).
		Send()

	return allDetections, nil
}

func (fs *FileScanner) scanFile(ctx context.Context, owner, repo, filePath string, publicImageMap map[string]*types.ImageInfo) ([]types.ImageDetectionResult, error) {
	fs.logger.Debug("scanning_file_for_images").
		Str("file", filePath).
		Int("public_images_to_check", len(publicImageMap)).
		Send()

	content, err := fs.githubClient.GetFileContent(ctx, owner, repo, filePath, "")
	if err != nil {
		fs.logger.Error("failed_to_get_file_content").
			Str("file", filePath).
			Err(err).
			Send()
		return nil, err
	}

	decodedContent, err := base64.StdEncoding.DecodeString(content.Content)
	if err != nil {
		fs.logger.Error("failed_to_decode_file_content").
			Str("file", filePath).
			Err(err).
			Send()
		return nil, fmt.Errorf("falha ao decodificar conteúdo: %w", err)
	}

	fileContent := string(decodedContent)
	fileType := fs.detectFileType(fileContent, filePath)

	fs.logger.Debug("file_analysis_info").
		Str("file", filePath).
		Str("detected_type", fs.fileTypeToString(fileType)).
		Int("content_size", len(fileContent)).
		Send()

	var detections []types.ImageDetectionResult

	switch fileType {
	case FileTypeKubernetesManifest:
		detections = fs.scanKubernetesManifest(fileContent, filePath, publicImageMap)
	case FileTypeHelmValues:
		detections = fs.scanHelmValues(fileContent, filePath, publicImageMap)
	case FileTypeArgoCDApplication:
		detections = fs.scanArgoCDApplication(fileContent, filePath, publicImageMap)
	case FileTypeKustomization:
		detections = fs.scanKustomization(fileContent, filePath, publicImageMap)
	default:
		detections = fs.scanGenericYAML(fileContent, filePath, publicImageMap)
	}

	for i := range detections {
		if detections[i].FilePath == "" {
			fs.logger.Warn("detection_missing_filepath_setting").
				Str("image", detections[i].FullImage).
				Str("file", filePath).
				Send()
			detections[i].FilePath = filePath
		}
	}

	fs.logger.Info("file_scan_completed").
		Str("file", filePath).
		Str("type", fs.fileTypeToString(fileType)).
		Int("detections", len(detections)).
		Send()

	for _, detection := range detections {
		fs.logger.Debug("detection_details").
			Str("file", detection.FilePath).
			Str("image", detection.FullImage).
			Int("line", detection.LineNumber).
			Float64("confidence", detection.Confidence).
			Send()
	}

	return detections, nil
}

func (fs *FileScanner) scanKubernetesManifest(content, filePath string, publicImageMap map[string]*types.ImageInfo) []types.ImageDetectionResult {
	var detections []types.ImageDetectionResult
	lines := strings.Split(content, "\n")

	for lineNum, line := range lines {
		if matches := imagePatterns["yaml_image"].FindStringSubmatch(line); len(matches) > 1 {
			imageName := matches[1]
			if _, isPublic := publicImageMap[imageName]; isPublic {
				detections = append(detections, types.ImageDetectionResult{
					Image:      imageName,
					Repository: fs.extractRepository(imageName),
					Tag:        fs.extractTag(imageName),
					Registry:   fs.extractRegistry(imageName),
					FullImage:  imageName,
					IsPublic:   true,
					LineNumber: lineNum + 1,
					Context:    strings.TrimSpace(line),
					Confidence: 1.0,
					FilePath:   filePath,
				})

				fs.logger.Debug("kubernetes_image_detected").
					Str("file", filePath).
					Str("image", imageName).
					Int("line", lineNum+1).
					Send()
			}
		}
	}

	return detections
}

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

func (fs *FileScanner) scanArgoCDApplication(content, filePath string, publicImageMap map[string]*types.ImageInfo) []types.ImageDetectionResult {
	var detections []types.ImageDetectionResult

	detections = append(detections, fs.scanGenericYAML(content, filePath, publicImageMap)...)

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
					})
				}
			}
		}
	}

	return detections
}

func (fs *FileScanner) scanKustomization(content, filePath string, publicImageMap map[string]*types.ImageInfo) []types.ImageDetectionResult {
	var detections []types.ImageDetectionResult
	lines := strings.Split(content, "\n")

	var currentNewName string
	var currentNewTag string

	for lineNum, line := range lines {
		if matches := imagePatterns["kustomize_newName"].FindStringSubmatch(line); len(matches) > 1 {
			currentNewName = matches[1]
		}

		if matches := imagePatterns["kustomize_newTag"].FindStringSubmatch(line); len(matches) > 1 {
			currentNewTag = matches[1]
		}

		if currentNewName != "" && currentNewTag != "" {
			fullImage := fmt.Sprintf("%s:%s", currentNewName, currentNewTag)

			if _, isPublic := publicImageMap[fullImage]; isPublic {
				detections = append(detections, types.ImageDetectionResult{
					Image:      fullImage,
					Repository: currentNewName,
					Tag:        currentNewTag,
					Registry:   fs.extractRegistry(currentNewName),
					FullImage:  fullImage,
					IsPublic:   true,
					LineNumber: lineNum + 1,
					Context:    fmt.Sprintf("newName: %s, newTag: %s", currentNewName, currentNewTag),
					Confidence: 0.9,
				})

				fs.logger.Debug("kustomize_image_detected").
					Str("file", filePath).
					Str("newName", currentNewName).
					Str("newTag", currentNewTag).
					Send()
			}

			currentNewName = ""
			currentNewTag = ""
		}
	}

	return detections
}

func (fs *FileScanner) scanGenericYAML(content, filePath string, publicImageMap map[string]*types.ImageInfo) []types.ImageDetectionResult {
	var detections []types.ImageDetectionResult
	lines := strings.Split(content, "\n")

	for lineNum, line := range lines {
		if matches := imagePatterns["yaml_image"].FindStringSubmatch(line); len(matches) > 1 {
			imageName := matches[1]
			if _, isPublic := publicImageMap[imageName]; isPublic {
				detections = append(detections, types.ImageDetectionResult{
					Image:      imageName,
					Repository: fs.extractRepository(imageName),
					Tag:        fs.extractTag(imageName),
					Registry:   fs.extractRegistry(imageName),
					FullImage:  imageName,
					IsPublic:   true,
					LineNumber: lineNum + 1,
					Context:    strings.TrimSpace(line),
					Confidence: 0.7,
				})
			}
		}
	}

	return detections
}

func (fs *FileScanner) detectFileType(content, filePath string) FileType {
	fileName := strings.ToLower(filePath)

	if strings.Contains(fileName, "values") && (strings.HasSuffix(fileName, ".yaml") || strings.HasSuffix(fileName, ".yml")) {
		return FileTypeHelmValues
	}

	if strings.Contains(fileName, "kustomization") {
		return FileTypeKustomization
	}

	if strings.Contains(fileName, "compose") {
		return FileTypeDockerCompose
	}

	for fileType, indicators := range fileTypeIndicators {
		matchCount := 0
		for _, indicator := range indicators {
			if strings.Contains(content, indicator) {
				matchCount++
			}
		}

		if matchCount >= len(indicators)/2 {
			return fileType
		}
	}

	return FileTypeUnknown
}

func (fs *FileScanner) createPublicImageMap(publicImages []*types.ImageInfo) map[string]*types.ImageInfo {
	imageMap := make(map[string]*types.ImageInfo)
	for _, img := range publicImages {
		imageMap[img.Image] = img

		if !strings.Contains(img.Image, ":") {
			imageMap[img.Image+":latest"] = img
		}
	}
	return imageMap
}

func (fs *FileScanner) extractRepository(imageName string) string {
	if strings.Contains(imageName, ":") {
		return strings.Split(imageName, ":")[0]
	}
	return imageName
}

func (fs *FileScanner) extractTag(imageName string) string {
	if strings.Contains(imageName, ":") {
		parts := strings.Split(imageName, ":")
		return parts[len(parts)-1]
	}
	return "latest"
}

func (fs *FileScanner) extractRegistry(imageName string) string {
	parts := strings.Split(imageName, "/")
	if len(parts) > 1 && strings.Contains(parts[0], ".") {
		return parts[0]
	}
	return "docker.io"
}

func (fs *FileScanner) findLineNumber(content, searchText string) int {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, searchText) {
			return i + 1
		}
	}
	return 0
}

func (fs *FileScanner) parseRepositoryName(repoName string) (owner, repo string, err error) {
	parts := strings.Split(repoName, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("formato de repositório inválido: %s", repoName)
	}
	return parts[0], parts[1], nil
}

func (fs *FileScanner) fileTypeToString(ft FileType) string {
	switch ft {
	case FileTypeKubernetesManifest:
		return "kubernetes_manifest"
	case FileTypeHelmValues:
		return "helm_values"
	case FileTypeArgoCDApplication:
		return "argocd_application"
	case FileTypeKustomization:
		return "kustomization"
	case FileTypeDockerCompose:
		return "docker_compose"
	default:
		return "unknown"
	}
}
