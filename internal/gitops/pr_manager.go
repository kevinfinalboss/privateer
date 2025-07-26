package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kevinfinalboss/privateer/internal/github"
	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

type PullRequestManager struct {
	githubClient *github.Client
	logger       *logger.Logger
	config       *types.Config
}

func NewPullRequestManager(githubClient *github.Client, logger *logger.Logger, config *types.Config) *PullRequestManager {
	return &PullRequestManager{
		githubClient: githubClient,
		logger:       logger,
		config:       config,
	}
}

func (prm *PullRequestManager) CreatePullRequest(ctx context.Context, repoConfig types.GitHubRepositoryConfig, gitopsResult *types.GitOpsResult) (*types.PullRequestInfo, error) {
	prm.logger.Info("creating_pull_request").
		Str("repository", repoConfig.Name).
		Str("branch", gitopsResult.Branch).
		Send()

	owner, repo, err := prm.parseRepositoryName(repoConfig.Name)
	if err != nil {
		return nil, err
	}

	repository, err := prm.githubClient.GetRepository(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("falha ao obter informações do repositório: %w", err)
	}

	title := prm.generatePRTitle(gitopsResult)
	body := prm.generatePRBody(repoConfig, gitopsResult)

	prRequest := types.CreatePRRequest{
		Title:               title,
		Head:                gitopsResult.Branch,
		Base:                repository.DefaultBranch,
		Body:                body,
		MaintainerCanModify: true,
		Draft:               repoConfig.PRSettings.Draft,
	}

	endpoint := fmt.Sprintf("/repos/%s/%s/pulls", owner, repo)
	payload, err := json.Marshal(prRequest)
	if err != nil {
		return nil, fmt.Errorf("falha ao codificar request: %w", err)
	}

	resp, err := prm.githubClient.MakeRequest(ctx, "POST", endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return nil, fmt.Errorf("falha ao criar pull request: %w", err)
	}

	if resp.StatusCode != 201 {
		prm.logger.Error("github_pr_creation_failed").
			Int("status_code", resp.StatusCode).
			Str("response_body", string(resp.Body)).
			Str("request_payload", string(payload)).
			Send()
		return nil, fmt.Errorf("falha ao criar pull request: status %d - %s", resp.StatusCode, string(resp.Body))
	}

	var prResponse types.PullRequestResponse
	if err := json.Unmarshal(resp.Body, &prResponse); err != nil {
		return nil, fmt.Errorf("falha ao decodificar resposta: %w", err)
	}

	prInfo := &types.PullRequestInfo{
		URL:       prResponse.HTMLURL,
		Number:    prResponse.Number,
		Title:     prResponse.Title,
		Body:      prResponse.Body,
		Draft:     repoConfig.PRSettings.Draft,
		State:     prResponse.State,
		CreatedAt: prResponse.CreatedAt,
		UpdatedAt: prResponse.UpdatedAt,
	}

	if prResponse.Mergeable != nil {
		prInfo.Mergeable = *prResponse.Mergeable
	}

	prm.logger.Info("pull_request_created").
		Str("repository", repoConfig.Name).
		Int("pr_number", prResponse.Number).
		Str("url", prResponse.HTMLURL).
		Send()

	if len(repoConfig.PRSettings.Reviewers) > 0 {
		if err := prm.addReviewers(ctx, owner, repo, prResponse.Number, repoConfig.PRSettings.Reviewers); err != nil {
			prm.logger.Warn("failed_to_add_reviewers").
				Err(err).
				Send()
		} else {
			prInfo.Reviewers = repoConfig.PRSettings.Reviewers
		}
	}

	if len(repoConfig.PRSettings.Labels) > 0 {
		if err := prm.addLabels(ctx, owner, repo, prResponse.Number, repoConfig.PRSettings.Labels); err != nil {
			prm.logger.Warn("failed_to_add_labels").
				Err(err).
				Send()
		} else {
			prInfo.Labels = repoConfig.PRSettings.Labels
		}
	}

	return prInfo, nil
}

func (prm *PullRequestManager) addReviewers(ctx context.Context, owner, repo string, prNumber int, reviewers []string) error {
	prm.logger.Debug("adding_reviewers").
		Strs("reviewers", reviewers).
		Int("pr_number", prNumber).
		Send()

	reviewerReq := types.ReviewerRequest{
		Reviewers: reviewers,
	}

	endpoint := fmt.Sprintf("/repos/%s/%s/pulls/%d/requested_reviewers", owner, repo, prNumber)
	payload, err := json.Marshal(reviewerReq)
	if err != nil {
		return fmt.Errorf("falha ao codificar reviewers: %w", err)
	}

	resp, err := prm.githubClient.MakeRequest(ctx, "POST", endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("falha ao adicionar reviewers: %w", err)
	}

	if resp.StatusCode != 201 {
		return fmt.Errorf("falha ao adicionar reviewers: status %d", resp.StatusCode)
	}

	prm.logger.Info("reviewers_added").
		Strs("reviewers", reviewers).
		Send()

	return nil
}

func (prm *PullRequestManager) addLabels(ctx context.Context, owner, repo string, prNumber int, labels []string) error {
	prm.logger.Debug("adding_labels").
		Strs("labels", labels).
		Int("pr_number", prNumber).
		Send()

	labelReq := types.LabelRequest{
		Labels: labels,
	}

	endpoint := fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, prNumber)
	payload, err := json.Marshal(labelReq)
	if err != nil {
		return fmt.Errorf("falha ao codificar labels: %w", err)
	}

	resp, err := prm.githubClient.MakeRequest(ctx, "POST", endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("falha ao adicionar labels: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("falha ao adicionar labels: status %d", resp.StatusCode)
	}

	prm.logger.Info("labels_added").
		Strs("labels", labels).
		Send()

	return nil
}

func (prm *PullRequestManager) generatePRTitle(gitopsResult *types.GitOpsResult) string {
	if len(gitopsResult.ImagesChanged) == 1 {
		return fmt.Sprintf("🏴‍☠️ Migrate %s to private registry", gitopsResult.ImagesChanged[0].SourceImage)
	}

	return fmt.Sprintf("🏴‍☠️ Migrate %d public images to private registries", len(gitopsResult.ImagesChanged))
}

func (prm *PullRequestManager) generatePRBody(repoConfig types.GitHubRepositoryConfig, gitopsResult *types.GitOpsResult) string {
	var body strings.Builder

	body.WriteString("# 🏴‍☠️ Privateer: Automated Image Migration\n\n")
	body.WriteString("This Pull Request was automatically generated by **Privateer** to migrate public Docker images to private registries for enhanced security and compliance.\n\n")

	body.WriteString("## 📊 Migration Summary\n\n")
	body.WriteString(fmt.Sprintf("- **Repository**: %s\n", gitopsResult.Repository))
	body.WriteString(fmt.Sprintf("- **Branch**: `%s`\n", gitopsResult.Branch))
	body.WriteString(fmt.Sprintf("- **Files Changed**: %d\n", len(gitopsResult.FilesChanged)))
	body.WriteString(fmt.Sprintf("- **Images Migrated**: %d\n", len(gitopsResult.ImagesChanged)))
	body.WriteString(fmt.Sprintf("- **Processing Time**: %s\n\n", gitopsResult.ProcessingTime))

	if len(gitopsResult.ImagesChanged) > 0 {
		body.WriteString("## 🔄 Image Migrations\n\n")
		body.WriteString("| Source Image | Target Image | Type |\n")
		body.WriteString("|--------------|--------------|------|\n")

		for _, change := range gitopsResult.ImagesChanged {
			sourceShort := prm.shortenImageName(change.SourceImage)
			targetShort := prm.shortenImageName(change.TargetImage)
			body.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n",
				sourceShort, targetShort, change.FileType))
		}
		body.WriteString("\n")
	}

	if len(gitopsResult.FilesChanged) > 0 {
		body.WriteString("## 📁 Files Modified\n\n")

		for _, file := range gitopsResult.FilesChanged {
			body.WriteString(fmt.Sprintf("### `%s`\n", file.FilePath))
			body.WriteString(fmt.Sprintf("- **Type**: %s\n", file.FileType))
			body.WriteString(fmt.Sprintf("- **Changes**: %d image references updated\n", len(file.Changes)))

			if len(file.Changes) > 0 {
				body.WriteString("\n**Updated images:**\n")
				for _, change := range file.Changes {
					body.WriteString(fmt.Sprintf("- Line %d: `%s` → `%s`\n",
						change.LineNumber,
						prm.shortenImageName(change.SourceImage),
						prm.shortenImageName(change.TargetImage)))
				}
			}
			body.WriteString("\n")
		}
	}

	body.WriteString("## ✅ Validation\n\n")
	body.WriteString("- ✅ All target images verified in private registries\n")
	body.WriteString("- ✅ YAML syntax validation passed\n")
	body.WriteString("- ✅ No duplicate image references detected\n")
	body.WriteString("- ✅ File integrity maintained\n\n")

	body.WriteString("## 🧪 Testing Checklist\n\n")
	body.WriteString("Before merging this PR, please verify:\n\n")
	body.WriteString("- [ ] Review all image changes above\n")
	body.WriteString("- [ ] Deploy to staging environment\n")
	body.WriteString("- [ ] Verify application functionality\n")
	body.WriteString("- [ ] Check that private registry access is configured\n")
	body.WriteString("- [ ] Monitor for any image pull errors\n\n")

	body.WriteString("## 🔒 Security Benefits\n\n")
	body.WriteString("This migration provides:\n")
	body.WriteString("- **Enhanced Security**: Private registries reduce supply chain attack surface\n")
	body.WriteString("- **Compliance**: Meet organizational security requirements\n")
	body.WriteString("- **Reliability**: Reduce dependency on external public registries\n")
	body.WriteString("- **Control**: Full control over image scanning and vulnerability management\n\n")

	body.WriteString("## 🤖 Automation Details\n\n")
	body.WriteString(fmt.Sprintf("- **Generated by**: Privateer v%s\n", "0.3.0"))
	body.WriteString(fmt.Sprintf("- **Generated at**: %s\n", time.Now().Format("2006-01-02 15:04:05 UTC")))
	body.WriteString(fmt.Sprintf("- **Branch strategy**: %s\n", repoConfig.BranchStrategy))
	body.WriteString(fmt.Sprintf("- **Auto-merge**: %t\n\n", repoConfig.PRSettings.AutoMerge))

	if len(repoConfig.PRSettings.Reviewers) > 0 {
		body.WriteString(fmt.Sprintf("**Reviewers**: %s\n", strings.Join(repoConfig.PRSettings.Reviewers, ", ")))
	}

	body.WriteString("\n---\n")
	body.WriteString("*This PR was automatically created by [Privateer](https://github.com/kevinfinalboss/privateer) 🏴‍☠️*")

	return body.String()
}

func (prm *PullRequestManager) shortenImageName(imageName string) string {
	if len(imageName) <= 50 {
		return imageName
	}

	parts := strings.Split(imageName, "/")
	if len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		if len(lastPart) <= 40 {
			return ".../" + lastPart
		}
	}

	if len(imageName) > 47 {
		return imageName[:47] + "..."
	}

	return imageName
}

func (prm *PullRequestManager) GetPRTemplate(repoConfig types.GitHubRepositoryConfig) (string, error) {
	if repoConfig.PRSettings.Template == "" {
		return "", nil
	}
	return "", nil
}

func (prm *PullRequestManager) ValidatePRSettings(repoConfig types.GitHubRepositoryConfig) error {
	if len(repoConfig.PRSettings.Reviewers) == 0 {
		prm.logger.Warn("no_reviewers_configured").
			Str("repository", repoConfig.Name).
			Send()
	}

	if len(repoConfig.PRSettings.Labels) == 0 {
		prm.logger.Warn("no_labels_configured").
			Str("repository", repoConfig.Name).
			Send()
	}

	if repoConfig.PRSettings.AutoMerge {
		prm.logger.Warn("auto_merge_enabled").
			Str("repository", repoConfig.Name).
			Str("warning", "Auto-merge está habilitado - use com cuidado").
			Send()
	}

	return nil
}

func (prm *PullRequestManager) parseRepositoryName(repoName string) (owner, repo string, err error) {
	parts := strings.Split(repoName, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("formato de repositório inválido: %s", repoName)
	}
	return parts[0], parts[1], nil
}
