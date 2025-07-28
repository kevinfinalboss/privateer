package gitops

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kevinfinalboss/privateer/internal/github"
	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/internal/registry"
	"github.com/kevinfinalboss/privateer/internal/scanner"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

type Engine struct {
	githubClient    *github.Client
	registryManager *registry.Manager
	fileScanner     *scanner.FileScanner
	logger          *logger.Logger
	config          *types.Config
	replacer        *ImageReplacer
	prManager       *PullRequestManager
	tagResolver     *TagResolver
}

func NewEngine(githubClient *github.Client, registryManager *registry.Manager, logger *logger.Logger, config *types.Config) *Engine {
	fileScanner := scanner.NewFileScanner(githubClient, logger, config)
	replacer := NewImageReplacer(logger, config)
	prManager := NewPullRequestManager(githubClient, logger, config)
	tagResolver := NewTagResolver(logger, config, registryManager)

	return &Engine{
		githubClient:    githubClient,
		registryManager: registryManager,
		fileScanner:     fileScanner,
		logger:          logger,
		config:          config,
		replacer:        replacer,
		prManager:       prManager,
		tagResolver:     tagResolver,
	}
}

func (e *Engine) MigrateRepositories(ctx context.Context, publicImages []*types.ImageInfo) (*types.GitOpsSummary, error) {
	startTime := time.Now()

	e.logger.Info("gitops_migration_started").
		Int("public_images", len(publicImages)).
		Bool("dry_run", e.config.Settings.DryRun).
		Bool("tag_resolution_enabled", e.config.GitOps.TagResolution.Enabled).
		Send()

	if !e.config.GitHub.Enabled {
		return nil, fmt.Errorf("GitHub não está habilitado na configuração")
	}

	if !e.config.GitOps.Enabled {
		return nil, fmt.Errorf("GitOps não está habilitado na configuração")
	}

	if err := e.githubClient.ValidateToken(ctx); err != nil {
		return nil, fmt.Errorf("falha na validação do token GitHub: %w", err)
	}

	enabledRepos := e.getEnabledRepositories()
	if len(enabledRepos) == 0 {
		return nil, fmt.Errorf("nenhum repositório GitHub habilitado encontrado")
	}

	if e.config.GitOps.TagResolution.Enabled {
		e.tagResolver.LoadClusterImages(publicImages)
		e.logger.Info("tag_resolver_loaded_with_cluster_images").
			Int("cluster_images", len(publicImages)).
			Send()
	}

	validatedImageMap, err := e.buildAndValidatePrivateImageMap(ctx, publicImages)
	if err != nil {
		return nil, fmt.Errorf("falha ao validar imagens nos registries privados: %w", err)
	}

	availableImages := e.filterValidatedImages(publicImages, validatedImageMap)
	if len(availableImages) == 0 {
		e.logger.Info("no_validated_images_available").
			Str("message", "Nenhuma imagem pública validada foi encontrada nos registries privados").
			Send()

		return &types.GitOpsSummary{
			TotalRepositories: len(enabledRepos),
			ProcessingTime:    time.Since(startTime).String(),
		}, nil
	}

	e.logger.Info("validated_images_for_gitops").
		Int("validated", len(availableImages)).
		Int("total_public", len(publicImages)).
		Send()

	summary := &types.GitOpsSummary{
		TotalRepositories: len(enabledRepos),
		Results:           make([]*types.GitOpsResult, 0),
		ProcessingTime:    time.Since(startTime).String(),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, e.config.Settings.Concurrency)

	for _, repoConfig := range enabledRepos {
		wg.Add(1)
		go func(repo types.GitHubRepositoryConfig) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := e.processRepository(ctx, repo, availableImages, validatedImageMap)

			mu.Lock()
			summary.Results = append(summary.Results, result)
			e.updateSummaryCounters(summary, result)
			mu.Unlock()
		}(repoConfig)
	}

	wg.Wait()

	summary.ProcessingTime = time.Since(startTime).String()

	e.logger.Info("gitops_migration_completed").
		Int("repositories_processed", summary.ProcessedRepositories).
		Int("successful_prs", summary.SuccessfulPRs).
		Int("failed_operations", summary.FailedOperations).
		Str("processing_time", summary.ProcessingTime).
		Send()

	return summary, nil
}

func (e *Engine) buildAndValidatePrivateImageMap(ctx context.Context, publicImages []*types.ImageInfo) (map[string]string, error) {
	e.logger.Info("building_and_validating_private_image_map").
		Int("public_images", len(publicImages)).
		Send()

	validatedImageMap, err := e.registryManager.ValidateImagesBatch(ctx, publicImages, e.config)
	if err != nil {
		return nil, fmt.Errorf("falha na validação em lote: %w", err)
	}

	e.logger.Info("image_validation_completed").
		Int("validated_images", len(validatedImageMap)).
		Int("total_public", len(publicImages)).
		Int("missing_images", len(publicImages)-len(validatedImageMap)).
		Send()

	if len(validatedImageMap) == 0 {
		e.logger.Warn("no_validated_images_found").
			Str("message", "Nenhuma imagem pública foi validada nos registries privados").
			Str("suggestion", "Execute 'privateer migrate cluster' primeiro para migrar as imagens").
			Send()
	}

	for publicImage, privateImage := range validatedImageMap {
		e.logger.Info("validated_mapping").
			Str("public", publicImage).
			Str("private", privateImage).
			Send()
	}

	return validatedImageMap, nil
}

func (e *Engine) filterValidatedImages(publicImages []*types.ImageInfo, validatedImageMap map[string]string) []*types.ImageInfo {
	var validated []*types.ImageInfo

	e.logger.Debug("filtering_validated_images").
		Int("total_public", len(publicImages)).
		Int("validated_map_size", len(validatedImageMap)).
		Send()

	for _, image := range publicImages {
		if _, exists := validatedImageMap[image.Image]; exists {
			validated = append(validated, image)
			e.logger.Debug("image_validated_for_gitops").
				Str("image", image.Image).
				Send()
		} else {
			e.logger.Debug("image_not_validated_for_gitops").
				Str("image", image.Image).
				Str("reason", "não validada em registry privado").
				Send()
		}
	}

	e.logger.Info("validated_images_filter_completed").
		Int("validated", len(validated)).
		Int("filtered_out", len(publicImages)-len(validated)).
		Send()

	return validated
}

func (e *Engine) processRepository(ctx context.Context, repoConfig types.GitHubRepositoryConfig, validatedImages []*types.ImageInfo, validatedImageMap map[string]string) *types.GitOpsResult {
	startTime := time.Now()

	e.logger.Info("processing_repository").
		Str("repository", repoConfig.Name).
		Int("validated_images", len(validatedImages)).
		Send()

	result := &types.GitOpsResult{
		Repository:     repoConfig.Name,
		FilesChanged:   make([]types.FileChange, 0),
		ImagesChanged:  make([]types.ImageReplacement, 0),
		Success:        false,
		ProcessingTime: "",
	}

	repoManager := github.NewRepositoryManager(e.githubClient)
	if err := repoManager.ValidateRepositoryAccess(ctx, repoConfig); err != nil {
		result.Error = fmt.Errorf("falha na validação do repositório: %w", err)
		return result
	}

	detections, err := e.fileScanner.ScanRepositoryForImages(ctx, repoConfig, validatedImages)
	if err != nil {
		result.Error = fmt.Errorf("falha no scan do repositório: %w", err)
		return result
	}

	if e.config.GitOps.TagResolution.Enabled {
		detections, err = e.tagResolver.ProcessDetections(ctx, detections)
		if err != nil {
			e.logger.Warn("tag_resolution_processing_failed").
				Str("repository", repoConfig.Name).
				Err(err).
				Send()
		} else {
			e.logger.Info("tag_resolution_processing_completed").
				Str("repository", repoConfig.Name).
				Int("processed_detections", len(detections)).
				Send()
		}
	}

	if len(detections) == 0 {
		e.logger.Info("no_validated_images_detected_in_repository").
			Str("repository", repoConfig.Name).
			Send()

		result.Success = true
		result.ProcessingTime = time.Since(startTime).String()
		return result
	}

	validatedReplacements := e.generateValidatedReplacements(detections, validatedImageMap)
	if len(validatedReplacements) == 0 {
		e.logger.Info("no_validated_replacements_needed").
			Str("repository", repoConfig.Name).
			Int("detections", len(detections)).
			Send()

		result.Success = true
		result.ProcessingTime = time.Since(startTime).String()
		return result
	}

	e.logger.Info("validated_replacements_generated").
		Str("repository", repoConfig.Name).
		Int("validated_replacements", len(validatedReplacements)).
		Int("total_detections", len(detections)).
		Send()

	for _, replacement := range validatedReplacements {
		e.logger.Debug("validated_replacement_detail").
			Str("source", replacement.SourceImage).
			Str("target", replacement.TargetImage).
			Str("file_path", replacement.FilePath).
			Str("type", replacement.FileType).
			Bool("validated", true).
			Send()
	}

	if e.config.Settings.DryRun {
		result = e.simulateRepositoryChanges(result, validatedReplacements)
		result.Success = true
		result.ProcessingTime = time.Since(startTime).String()
		return result
	}

	branchName := repoManager.GenerateBranchName(e.config.GitOps.BranchPrefix, fmt.Sprintf("%d-images", len(validatedReplacements)))

	owner, repo, err := e.parseRepositoryName(repoConfig.Name)
	if err != nil {
		result.Error = err
		return result
	}

	_, defaultSHA, err := repoManager.GetDefaultBranch(ctx, owner, repo)
	if err != nil {
		result.Error = fmt.Errorf("falha ao obter branch padrão: %w", err)
		return result
	}

	_, err = repoManager.CreateBranch(ctx, owner, repo, branchName, defaultSHA)
	if err != nil {
		result.Error = fmt.Errorf("falha ao criar branch: %w", err)
		return result
	}

	result.Branch = branchName

	fileChanges, err := e.applyValidatedReplacements(ctx, owner, repo, branchName, validatedReplacements)
	if err != nil {
		result.Error = fmt.Errorf("falha ao aplicar mudanças validadas: %w", err)
		return result
	}

	result.FilesChanged = fileChanges
	result.ImagesChanged = validatedReplacements

	if e.config.GitOps.AutoPR {
		prInfo, err := e.prManager.CreatePullRequest(ctx, repoConfig, result)
		if err != nil {
			e.logger.Error("pull_request_creation_failed").
				Str("repository", repoConfig.Name).
				Err(err).
				Send()
			result.Error = fmt.Errorf("falha ao criar pull request: %w", err)
			return result
		}
		result.PullRequest = prInfo
	}

	result.Success = true
	result.ProcessingTime = time.Since(startTime).String()

	e.logger.Info("repository_processed_successfully_with_validation").
		Str("repository", repoConfig.Name).
		Str("branch", branchName).
		Int("files_changed", len(fileChanges)).
		Int("validated_images_replaced", len(validatedReplacements)).
		Send()

	return result
}

func (e *Engine) generateValidatedReplacements(detections []types.ImageDetectionResult, validatedImageMap map[string]string) []types.ImageReplacement {
	var replacements []types.ImageReplacement

	e.logger.Debug("generating_validated_replacements").
		Int("detections", len(detections)).
		Int("validated_images_available", len(validatedImageMap)).
		Send()

	for _, detection := range detections {
		e.logger.Debug("processing_detection_for_validation").
			Str("image", detection.FullImage).
			Str("file_path", detection.FilePath).
			Bool("file_path_empty", detection.FilePath == "").
			Int("line_number", detection.LineNumber).
			Send()

		if detection.FilePath == "" {
			e.logger.Warn("detection_missing_filepath_skipping").
				Str("image", detection.FullImage).
				Str("context", detection.Context).
				Send()
			continue
		}

		if validatedPrivateImage, exists := validatedImageMap[detection.FullImage]; exists {
			replacement := types.ImageReplacement{
				SourceImage:    detection.FullImage,
				TargetImage:    validatedPrivateImage,
				FileType:       e.detectReplacementType(detection.Context),
				FilePath:       detection.FilePath,
				LineNumber:     detection.LineNumber,
				Context:        detection.Context,
				ReplacementKey: e.generateReplacementKey(detection),
			}

			replacements = append(replacements, replacement)

			e.logger.Debug("validated_replacement_created").
				Str("source", replacement.SourceImage).
				Str("target", replacement.TargetImage).
				Str("file_path", replacement.FilePath).
				Str("file_type", replacement.FileType).
				Bool("validated", true).
				Send()
		} else {
			e.logger.Debug("image_not_validated_skipping").
				Str("public_image", detection.FullImage).
				Str("message", "Imagem não validada - não será incluída no replacement").
				Send()
		}
	}

	e.logger.Info("validated_replacements_generation_completed").
		Int("total_validated_replacements", len(replacements)).
		Int("skipped_detections", len(detections)-len(replacements)).
		Send()

	return replacements
}

func (e *Engine) applyValidatedReplacements(ctx context.Context, owner, repo, branch string, validatedReplacements []types.ImageReplacement) ([]types.FileChange, error) {
	e.logger.Info("applying_validated_replacements").
		Str("owner", owner).
		Str("repo", repo).
		Str("branch", branch).
		Int("validated_replacements", len(validatedReplacements)).
		Send()

	fileReplacements := e.groupValidatedReplacementsByFile(validatedReplacements)
	if len(fileReplacements) == 0 {
		e.logger.Warn("no_validated_files_to_process").
			Str("message", "Nenhum arquivo para processar após validação e agrupamento").
			Send()
		return []types.FileChange{}, nil
	}

	var fileChanges []types.FileChange
	repoManager := github.NewRepositoryManager(e.githubClient)

	for filePath, fileReplacements := range fileReplacements {
		e.logger.Debug("processing_validated_file").
			Str("file", filePath).
			Int("validated_replacements", len(fileReplacements)).
			Send()

		content, err := e.githubClient.GetFileContent(ctx, owner, repo, filePath, "")
		if err != nil {
			e.logger.Error("failed_to_fetch_file").
				Str("file", filePath).
				Err(err).
				Send()
			return nil, fmt.Errorf("falha ao obter conteúdo do arquivo %s: %w", filePath, err)
		}

		originalContent, err := base64.StdEncoding.DecodeString(content.Content)
		if err != nil {
			e.logger.Error("failed_to_decode_file").
				Str("file", filePath).
				Err(err).
				Send()
			return nil, fmt.Errorf("falha ao decodificar arquivo %s: %w", filePath, err)
		}

		modifiedContent, actualReplacements, err := e.replacer.ReplaceImagesInContent(string(originalContent), fileReplacements)
		if err != nil {
			e.logger.Error("failed_to_replace_validated_images").
				Str("file", filePath).
				Err(err).
				Send()
			return nil, fmt.Errorf("falha ao substituir imagens validadas no arquivo %s: %w", filePath, err)
		}

		e.logger.Debug("validated_replacement_completed").
			Str("file", filePath).
			Int("actual_replacements", len(actualReplacements)).
			Bool("content_changed", modifiedContent != string(originalContent)).
			Send()

		if modifiedContent == string(originalContent) {
			e.logger.Warn("no_validated_changes_made").
				Str("file", filePath).
				Str("message", "Conteúdo não foi alterado após substituições validadas").
				Send()
			continue
		}

		encodedContent := base64.StdEncoding.EncodeToString([]byte(modifiedContent))
		commitMessage := e.generateCommitMessage(fileReplacements)

		_, err = repoManager.UpdateFile(ctx, owner, repo, filePath, encodedContent, commitMessage, branch)
		if err != nil {
			e.logger.Error("failed_to_update_file_with_validated_changes").
				Str("file", filePath).
				Err(err).
				Send()
			return nil, fmt.Errorf("falha ao atualizar arquivo %s: %w", filePath, err)
		}

		fileChange := types.FileChange{
			FilePath:      filePath,
			FileType:      e.detectFileType(filePath),
			Changes:       actualReplacements,
			LinesChanged:  len(actualReplacements),
			Validated:     true,
			BackupContent: string(originalContent),
		}

		fileChanges = append(fileChanges, fileChange)

		e.logger.Info("validated_file_updated_successfully").
			Str("file", filePath).
			Int("validated_replacements", len(actualReplacements)).
			Send()
	}

	e.logger.Info("apply_validated_replacements_completed").
		Int("files_changed", len(fileChanges)).
		Send()

	return fileChanges, nil
}

func (e *Engine) groupValidatedReplacementsByFile(validatedReplacements []types.ImageReplacement) map[string][]types.ImageReplacement {
	fileMap := make(map[string][]types.ImageReplacement)

	e.logger.Debug("grouping_validated_replacements_by_file").
		Int("total_validated_replacements", len(validatedReplacements)).
		Send()

	for _, replacement := range validatedReplacements {
		e.logger.Debug("processing_validated_replacement_for_grouping").
			Str("source", replacement.SourceImage).
			Str("target", replacement.TargetImage).
			Str("file_path", replacement.FilePath).
			Bool("file_path_empty", replacement.FilePath == "").
			Send()

		if replacement.FilePath == "" {
			e.logger.Warn("validated_replacement_missing_filepath").
				Str("source", replacement.SourceImage).
				Str("target", replacement.TargetImage).
				Str("context", replacement.Context).
				Send()
			continue
		}
		fileMap[replacement.FilePath] = append(fileMap[replacement.FilePath], replacement)
	}

	e.logger.Info("validated_file_grouping_completed").
		Int("files_to_update", len(fileMap)).
		Send()

	for filePath, fileReplacements := range fileMap {
		e.logger.Debug("validated_file_group_details").
			Str("file_path", filePath).
			Int("validated_replacements_count", len(fileReplacements)).
			Send()
	}

	return fileMap
}

func (e *Engine) detectReplacementType(context string) string {
	context = strings.ToLower(context)

	if strings.Contains(context, "registry:") && strings.Contains(context, "repository:") && strings.Contains(context, "tag:") {
		return "helm_separated"
	} else if strings.Contains(context, "repository:") && strings.Contains(context, "tag:") && strings.Contains(context, "(combined)") {
		return "helm_combined"
	} else if strings.Contains(context, "newname:") {
		return "kustomize"
	} else if strings.Contains(context, "image:") {
		return "kubernetes_manifest"
	}

	return "generic"
}

func (e *Engine) generateReplacementKey(detection types.ImageDetectionResult) string {
	return fmt.Sprintf("%s:%d", detection.Image, detection.LineNumber)
}

func (e *Engine) generateCommitMessage(replacements []types.ImageReplacement) string {
	if len(replacements) == 1 {
		return strings.ReplaceAll(e.config.GitOps.CommitMessage, "{image}", replacements[0].SourceImage)
	}

	return fmt.Sprintf("%s (%d validated images)",
		strings.ReplaceAll(e.config.GitOps.CommitMessage, "{image}", "multiple"),
		len(replacements))
}

func (e *Engine) detectFileType(filePath string) string {
	fileName := strings.ToLower(filePath)

	if strings.Contains(fileName, "values") {
		return "helm_values"
	} else if strings.Contains(fileName, "kustomization") {
		return "kustomization"
	} else if strings.Contains(fileName, "application") {
		return "argocd_application"
	}

	return "kubernetes_manifest"
}

func (e *Engine) simulateRepositoryChanges(result *types.GitOpsResult, validatedReplacements []types.ImageReplacement) *types.GitOpsResult {
	e.logger.Info("simulating_validated_repository_changes").
		Str("repository", result.Repository).
		Int("validated_replacements", len(validatedReplacements)).
		Send()

	fileMap := make(map[string][]types.ImageReplacement)

	for _, replacement := range validatedReplacements {
		if replacement.FilePath == "" {
			fallbackPath := e.generateFallbackFilePath(replacement.FileType)
			replacement.FilePath = fallbackPath
		}
		fileMap[replacement.FilePath] = append(fileMap[replacement.FilePath], replacement)
	}

	for filePath, fileReplacements := range fileMap {
		fileChange := types.FileChange{
			FilePath:     filePath,
			FileType:     e.detectFileType(filePath),
			Changes:      fileReplacements,
			LinesChanged: len(fileReplacements),
			Validated:    true,
		}

		result.FilesChanged = append(result.FilesChanged, fileChange)

		e.logger.Info("simulated_validated_file_change").
			Str("file", filePath).
			Int("validated_changes", len(fileReplacements)).
			Send()
	}

	result.ImagesChanged = validatedReplacements
	result.Branch = e.config.GitOps.BranchPrefix + "simulation-validated"

	return result
}

func (e *Engine) generateFallbackFilePath(fileType string) string {
	switch fileType {
	case "helm_separated":
		return "values.yaml"
	case "kustomize":
		return "kustomization.yaml"
	case "argocd_application":
		return "application.yaml"
	case "kubernetes_manifest":
		return "deployment.yaml"
	case "helm_values":
		return "values.yaml"
	default:
		return "manifest.yaml"
	}
}

func (e *Engine) getEnabledRepositories() []types.GitHubRepositoryConfig {
	var enabled []types.GitHubRepositoryConfig

	for _, repo := range e.config.GitHub.Repositories {
		if repo.Enabled {
			enabled = append(enabled, repo)
		}
	}

	sort.Slice(enabled, func(i, j int) bool {
		return enabled[i].Priority > enabled[j].Priority
	})

	return enabled
}

func (e *Engine) updateSummaryCounters(summary *types.GitOpsSummary, result *types.GitOpsResult) {
	summary.ProcessedRepositories++

	if result.Success {
		if result.PullRequest != nil {
			summary.SuccessfulPRs++
		}
		summary.TotalFilesChanged += len(result.FilesChanged)
		summary.TotalImagesReplaced += len(result.ImagesChanged)
	} else {
		summary.FailedOperations++
		if result.Error != nil {
			summary.Errors = append(summary.Errors, result.Error)
		}
	}
}

func (e *Engine) parseRepositoryName(repoName string) (owner, repo string, err error) {
	parts := strings.Split(repoName, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("formato de repositório inválido: %s", repoName)
	}
	return parts[0], parts[1], nil
}
