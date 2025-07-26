package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

const (
	GitHubAPIURL   = "https://api.github.com"
	DefaultTimeout = 30 * time.Second
	MaxRetries     = 3
	RetryDelay     = 2 * time.Second
)

type Client struct {
	token      string
	httpClient *http.Client
	logger     *logger.Logger
	config     *types.GitHubConfig
}

func NewClient(config *types.GitHubConfig, logger *logger.Logger) *Client {
	return &Client{
		token: config.Token,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		logger: logger,
		config: config,
	}
}

func (c *Client) ValidateToken(ctx context.Context) error {
	c.logger.Debug("github_token_validation").Send()

	resp, err := c.makeRequest(ctx, "GET", "/user", nil)
	if err != nil {
		c.logger.Error("github_token_invalid").Err(err).Send()
		return fmt.Errorf("token GitHub inválido: %w", err)
	}

	if resp.StatusCode == 401 {
		return fmt.Errorf("token GitHub não autorizado - verifique permissões")
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("falha na validação do token GitHub: status %d", resp.StatusCode)
	}

	var user map[string]interface{}
	if err := json.Unmarshal(resp.Body, &user); err != nil {
		return fmt.Errorf("falha ao decodificar resposta do GitHub: %w", err)
	}

	login, ok := user["login"].(string)
	if !ok {
		return fmt.Errorf("resposta inválida da API GitHub")
	}

	c.logger.Info("github_token_valid").
		Str("user", login).
		Send()

	return nil
}

func (c *Client) GetRepository(ctx context.Context, owner, repo string) (*types.Repository, error) {
	c.logger.Debug("github_get_repository").
		Str("owner", owner).
		Str("repo", repo).
		Send()

	endpoint := fmt.Sprintf("/repos/%s/%s", owner, repo)
	resp, err := c.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("repositório %s/%s não encontrado", owner, repo)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("falha ao obter repositório: status %d", resp.StatusCode)
	}

	var repository types.Repository
	if err := json.Unmarshal(resp.Body, &repository); err != nil {
		return nil, fmt.Errorf("falha ao decodificar repositório: %w", err)
	}

	c.logger.Debug("github_repository_found").
		Str("full_name", repository.FullName).
		Str("default_branch", repository.DefaultBranch).
		Bool("private", repository.Private).
		Send()

	return &repository, nil
}

func (c *Client) ListBranches(ctx context.Context, owner, repo string) ([]types.Branch, error) {
	c.logger.Debug("github_list_branches").
		Str("owner", owner).
		Str("repo", repo).
		Send()

	endpoint := fmt.Sprintf("/repos/%s/%s/branches", owner, repo)
	resp, err := c.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("falha ao listar branches: status %d", resp.StatusCode)
	}

	var branches []types.Branch
	if err := json.Unmarshal(resp.Body, &branches); err != nil {
		return nil, fmt.Errorf("falha ao decodificar branches: %w", err)
	}

	c.logger.Debug("github_branches_found").
		Int("count", len(branches)).
		Send()

	return branches, nil
}

func (c *Client) GetFileContent(ctx context.Context, owner, repo, path, ref string) (*types.FileContent, error) {
	c.logger.Debug("github_get_file").
		Str("owner", owner).
		Str("repo", repo).
		Str("path", path).
		Str("ref", ref).
		Send()

	endpoint := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, path)
	if ref != "" {
		endpoint += "?ref=" + ref
	}

	resp, err := c.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("arquivo %s não encontrado", path)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("falha ao obter arquivo: status %d", resp.StatusCode)
	}

	var content types.FileContent
	if err := json.Unmarshal(resp.Body, &content); err != nil {
		return nil, fmt.Errorf("falha ao decodificar conteúdo: %w", err)
	}

	return &content, nil
}

func (c *Client) GetTree(ctx context.Context, owner, repo, sha string, recursive bool) (*types.Tree, error) {
	c.logger.Debug("github_get_tree").
		Str("owner", owner).
		Str("repo", repo).
		Str("sha", sha).
		Bool("recursive", recursive).
		Send()

	endpoint := fmt.Sprintf("/repos/%s/%s/git/trees/%s", owner, repo, sha)
	if recursive {
		endpoint += "?recursive=1"
	}

	resp, err := c.makeRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("falha ao obter tree: status %d", resp.StatusCode)
	}

	var tree types.Tree
	if err := json.Unmarshal(resp.Body, &tree); err != nil {
		return nil, fmt.Errorf("falha ao decodificar tree: %w", err)
	}

	c.logger.Debug("github_tree_loaded").
		Int("files", len(tree.Tree)).
		Send()

	return &tree, nil
}

func (c *Client) CheckPermissions(ctx context.Context, owner, repo string) (*types.Permissions, error) {
	repository, err := c.GetRepository(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	if repository.Permissions.Push {
		c.logger.Debug("github_permissions_ok").
			Str("repo", repository.FullName).
			Send()
	} else {
		c.logger.Warn("github_no_push_permission").
			Str("repo", repository.FullName).
			Send()
	}

	return &repository.Permissions, nil
}

func (c *Client) makeRequest(ctx context.Context, method, endpoint string, body io.Reader) (*types.GitHubResponse, error) {
	url := GitHubAPIURL + endpoint

	var lastErr error
	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			c.logger.Debug("github_retry_request").
				Int("attempt", attempt+1).
				Str("endpoint", endpoint).
				Send()

			select {
			case <-time.After(RetryDelay * time.Duration(attempt)):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("Authorization", "token "+c.token)
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		req.Header.Set("User-Agent", "Privateer/1.0")

		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		responseBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == 403 {
			rateLimitRemaining := resp.Header.Get("X-RateLimit-Remaining")
			if rateLimitRemaining == "0" {
				resetTime := resp.Header.Get("X-RateLimit-Reset")
				c.logger.Warn("github_rate_limit").
					Str("reset_time", resetTime).
					Send()

				if attempt < MaxRetries-1 {
					continue
				}
			}
		}

		c.logger.Debug("github_request_completed").
			Str("method", method).
			Str("endpoint", endpoint).
			Int("status", resp.StatusCode).
			Int("attempt", attempt+1).
			Send()

		return &types.GitHubResponse{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Body:       responseBody,
		}, nil
	}

	return nil, fmt.Errorf("falha após %d tentativas: %w", MaxRetries, lastErr)
}

func (c *Client) MakeRequest(ctx context.Context, method, endpoint string, body io.Reader) (*types.GitHubResponse, error) {
	return c.makeRequest(ctx, method, endpoint, body)
}

func (c *Client) ParseRepositoryName(repoName string) (owner, repo string, err error) {
	return c.parseRepositoryName(repoName)
}

func (c *Client) parseRepositoryName(repoName string) (owner, repo string, err error) {
	parts := strings.Split(repoName, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("formato de repositório inválido: %s (deve ser owner/repo)", repoName)
	}
	return parts[0], parts[1], nil
}

func (c *Client) IsConfigured() bool {
	return c.config.Enabled && c.token != "" && len(c.config.Repositories) > 0
}
