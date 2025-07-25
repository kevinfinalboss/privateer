package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kevinfinalboss/privateer/pkg/types"
)

type RepositoryManager struct {
	client *Client
}

type CreateBranchRequest struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

type UpdateFileRequest struct {
	Message   string     `json:"message"`
	Content   string     `json:"content"`
	SHA       string     `json:"sha,omitempty"`
	Branch    string     `json:"branch,omitempty"`
	Committer *Committer `json:"committer,omitempty"`
}

type Committer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UpdateFileResponse struct {
	Content struct {
		SHA  string `json:"sha"`
		Path string `json:"path"`
	} `json:"content"`
	Commit struct {
		SHA     string `json:"sha"`
		Message string `json:"message"`
	} `json:"commit"`
}

func NewRepositoryManager(client *Client) *RepositoryManager {
	return &RepositoryManager{
		client: client,
	}
}

func (rm *RepositoryManager) CreateBranch(ctx context.Context, owner, repo, branchName, baseSHA string) (*types.BranchOperation, error) {
	rm.client.logger.Debug("github_create_branch").
		Str("owner", owner).
		Str("repo", repo).
		Str("branch", branchName).
		Str("base_sha", baseSHA).
		Send()

	if exists, err := rm.branchExists(ctx, owner, repo, branchName); err != nil {
		return nil, err
	} else if exists {
		rm.client.logger.Info("github_branch_exists").
			Str("branch", branchName).
			Send()

		return &types.BranchOperation{
			Repository:   fmt.Sprintf("%s/%s", owner, repo),
			BaseBranch:   "main",
			TargetBranch: branchName,
			Created:      false,
			Exists:       true,
		}, nil
	}

	endpoint := fmt.Sprintf("/repos/%s/%s/git/refs", owner, repo)
	payload := CreateBranchRequest{
		Ref: "refs/heads/" + branchName,
		SHA: baseSHA,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("falha ao codificar payload: %w", err)
	}

	resp, err := rm.client.MakeRequest(ctx, "POST", endpoint, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("falha ao criar branch: status %d", resp.StatusCode)
	}

	rm.client.logger.Info("github_branch_created").
		Str("owner", owner).
		Str("repo", repo).
		Str("branch", branchName).
		Send()

	return &types.BranchOperation{
		Repository:   fmt.Sprintf("%s/%s", owner, repo),
		BaseBranch:   "main",
		TargetBranch: branchName,
		Created:      true,
		Exists:       false,
		CommitSHA:    baseSHA,
	}, nil
}

func (rm *RepositoryManager) branchExists(ctx context.Context, owner, repo, branchName string) (bool, error) {
	branches, err := rm.client.ListBranches(ctx, owner, repo)
	if err != nil {
		return false, err
	}

	for _, branch := range branches {
		if branch.Name == branchName {
			return true, nil
		}
	}

	return false, nil
}

func (rm *RepositoryManager) GetDefaultBranch(ctx context.Context, owner, repo string) (string, string, error) {
	repository, err := rm.client.GetRepository(ctx, owner, repo)
	if err != nil {
		return "", "", err
	}

	branches, err := rm.client.ListBranches(ctx, owner, repo)
	if err != nil {
		return "", "", err
	}

	var defaultSHA string
	for _, branch := range branches {
		if branch.Name == repository.DefaultBranch {
			defaultSHA = branch.Commit.SHA
			break
		}
	}

	if defaultSHA == "" {
		return "", "", fmt.Errorf("não foi possível encontrar SHA da branch padrão")
	}

	rm.client.logger.Debug("github_default_branch").
		Str("branch", repository.DefaultBranch).
		Str("sha", defaultSHA).
		Send()

	return repository.DefaultBranch, defaultSHA, nil
}

func (rm *RepositoryManager) UpdateFile(ctx context.Context, owner, repo, path, content, message, branch string) (*UpdateFileResponse, error) {
	rm.client.logger.Debug("github_update_file").
		Str("owner", owner).
		Str("repo", repo).
		Str("path", path).
		Str("branch", branch).
		Send()

	existingFile, err := rm.client.GetFileContent(ctx, owner, repo, path, branch)
	var existingSHA string
	if err == nil {
		existingSHA = existingFile.SHA
	}

	endpoint := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path)

	payload := UpdateFileRequest{
		Message: message,
		Content: content,
		Branch:  branch,
		Committer: &Committer{
			Name:  "Privateer Bot",
			Email: "privateer@devops.local",
		},
	}

	if existingSHA != "" {
		payload.SHA = existingSHA
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("falha ao codificar payload: %w", err)
	}

	resp, err := rm.client.MakeRequest(ctx, "PUT", endpoint, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("falha ao atualizar arquivo: status %d", resp.StatusCode)
	}

	var updateResp UpdateFileResponse
	if err := json.Unmarshal(resp.Body, &updateResp); err != nil {
		return nil, fmt.Errorf("falha ao decodificar resposta: %w", err)
	}

	rm.client.logger.Info("github_file_updated").
		Str("path", path).
		Str("commit_sha", updateResp.Commit.SHA).
		Send()

	return &updateResp, nil
}

func (rm *RepositoryManager) ListRepositoryFiles(ctx context.Context, repoConfig types.GitHubRepositoryConfig) ([]TreeEntry, error) {
	owner, repo, err := parseRepositoryName(repoConfig.Name)
	if err != nil {
		return nil, err
	}

	_, defaultSHA, err := rm.GetDefaultBranch(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	tree, err := rm.client.GetTree(ctx, owner, repo, defaultSHA, true)
	if err != nil {
		return nil, err
	}

	var relevantFiles []TreeEntry
	for _, entry := range tree.Tree {
		if entry.Type != "blob" {
			continue
		}

		if rm.shouldIncludeFile(entry.Path, repoConfig) {
			relevantFiles = append(relevantFiles, entry)
		}
	}

	rm.client.logger.Debug("github_files_filtered").
		Str("repo", repoConfig.Name).
		Int("total_files", len(tree.Tree)).
		Int("relevant_files", len(relevantFiles)).
		Send()

	return relevantFiles, nil
}

func (rm *RepositoryManager) shouldIncludeFile(filePath string, repoConfig types.GitHubRepositoryConfig) bool {
	for _, excludedPath := range repoConfig.ExcludedPaths {
		if strings.HasPrefix(filePath, excludedPath) {
			return false
		}
	}

	if len(repoConfig.Paths) == 0 {
		return true
	}

	for _, includePath := range repoConfig.Paths {
		if strings.HasPrefix(filePath, includePath) {
			return true
		}

		if rm.matchesPattern(filePath, includePath) {
			return true
		}
	}

	return false
}

func (rm *RepositoryManager) matchesPattern(filePath, pattern string) bool {
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(filePath, parts[0]) && strings.HasSuffix(filePath, parts[1])
		}
	}
	return false
}

func (rm *RepositoryManager) ValidateRepositoryAccess(ctx context.Context, repoConfig types.GitHubRepositoryConfig) error {
	owner, repo, err := parseRepositoryName(repoConfig.Name)
	if err != nil {
		return err
	}

	permissions, err := rm.client.CheckPermissions(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("falha ao verificar permissões do repositório %s: %w", repoConfig.Name, err)
	}

	if !permissions.Pull {
		return fmt.Errorf("sem permissão de leitura no repositório %s", repoConfig.Name)
	}

	if !permissions.Push {
		rm.client.logger.Warn("github_no_push_permission").
			Str("repo", repoConfig.Name).
			Str("message", "Sem permissão de escrita - PRs podem falhar").
			Send()
	}

	rm.client.logger.Info("github_repository_validated").
		Str("repo", repoConfig.Name).
		Bool("can_read", permissions.Pull).
		Bool("can_write", permissions.Push).
		Send()

	return nil
}

func (rm *RepositoryManager) GenerateBranchName(prefix, imageInfo string) string {
	cleanImage := strings.ReplaceAll(imageInfo, "/", "-")
	cleanImage = strings.ReplaceAll(cleanImage, ":", "-")
	cleanImage = strings.ReplaceAll(cleanImage, ".", "-")

	if len(cleanImage) > 30 {
		cleanImage = cleanImage[:30]
	}

	timestamp := time.Now().Format("20060102-150405")

	return fmt.Sprintf("%s%s-%s", prefix, cleanImage, timestamp)
}

func (rm *RepositoryManager) GetFilesByExtension(files []TreeEntry, extensions []string) []TreeEntry {
	var filtered []TreeEntry

	for _, file := range files {
		for _, ext := range extensions {
			if strings.HasSuffix(strings.ToLower(file.Path), "."+ext) {
				filtered = append(filtered, file)
				break
			}
		}
	}

	return filtered
}

func parseRepositoryName(repoName string) (owner, repo string, err error) {
	parts := strings.Split(repoName, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("formato de repositório inválido: %s", repoName)
	}
	return parts[0], parts[1], nil
}
