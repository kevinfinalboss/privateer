package registry

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

type GHCRRegistry struct {
	*BaseRegistry
	Organization string
	httpClient   *http.Client
}

func NewGHCRRegistry(config *types.RegistryConfig, logger *logger.Logger) (*GHCRRegistry, error) {
	base := &BaseRegistry{
		Name:     config.Name,
		Type:     "ghcr",
		Logger:   logger,
		Username: config.Username,
		Password: config.Password,
		URL:      "ghcr.io",
		Insecure: false,
	}

	organization := config.Username
	if config.Project != "" {
		organization = config.Project
	}

	httpClient := createHTTPClient(false)

	return &GHCRRegistry{
		BaseRegistry: base,
		Organization: organization,
		httpClient:   httpClient,
	}, nil
}

func (r *GHCRRegistry) Login(ctx context.Context) error {
	r.Logger.Debug("ghcr_login_start").
		Str("registry", r.Name).
		Str("organization", r.Organization).
		Send()

	cmd := exec.CommandContext(ctx, "docker", "login", "ghcr.io", "-u", r.Username, "--password-stdin")
	cmd.Stdin = strings.NewReader(r.Password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("ghcr_login_failed").
			Str("registry", r.Name).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha no login do GHCR %s: %w", r.Name, err)
	}

	r.Logger.Info("ghcr_login_success").
		Str("registry", r.Name).
		Send()

	return nil
}

func (r *GHCRRegistry) Pull(ctx context.Context, imageName string) error {
	r.Logger.Debug("ghcr_pull_start").
		Str("image", imageName).
		Send()

	cmd := exec.CommandContext(ctx, "docker", "pull", imageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("ghcr_pull_failed").
			Str("image", imageName).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer pull da imagem %s: %w", imageName, err)
	}

	r.Logger.Info("ghcr_pull_success").
		Str("image", imageName).
		Send()

	return nil
}

func (r *GHCRRegistry) Push(ctx context.Context, image *types.ImageInfo, targetTag string) error {
	r.Logger.Debug("ghcr_push_start").
		Str("source", image.Image).
		Str("target", targetTag).
		Send()

	cmd := exec.CommandContext(ctx, "docker", "tag", image.Image, targetTag)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("ghcr_tag_failed").
			Str("source", image.Image).
			Str("target", targetTag).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer tag da imagem: %w", err)
	}

	cmd = exec.CommandContext(ctx, "docker", "push", targetTag)
	output, err = cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("ghcr_push_failed").
			Str("target", targetTag).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer push da imagem %s: %w", targetTag, err)
	}

	r.Logger.Info("ghcr_push_success").
		Str("target", targetTag).
		Send()

	return nil
}

func (r *GHCRRegistry) Copy(ctx context.Context, sourceImage, targetImage string) error {
	r.Logger.Debug("ghcr_copy_start").
		Str("source", sourceImage).
		Str("target", targetImage).
		Send()

	if err := r.Pull(ctx, sourceImage); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "docker", "tag", sourceImage, targetImage)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("ghcr_tag_failed").
			Str("source", sourceImage).
			Str("target", targetImage).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer tag da imagem: %w", err)
	}

	cmd = exec.CommandContext(ctx, "docker", "push", targetImage)
	output, err = cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("ghcr_push_failed").
			Str("target", targetImage).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer push da imagem %s: %w", targetImage, err)
	}

	r.Logger.Info("ghcr_copy_success").
		Str("source", sourceImage).
		Str("target", targetImage).
		Send()

	return nil
}

func (r *GHCRRegistry) IsHealthy(ctx context.Context) error {
	url := "https://ghcr.io/v2/"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	if r.Username != "" && r.Password != "" {
		req.SetBasicAuth(r.Username, r.Password)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("falha na conexão com GHCR: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("GHCR retornou status %d", resp.StatusCode)
	}

	return nil
}

func (r *GHCRRegistry) HasImage(ctx context.Context, imageName string) (bool, error) {
	parts := strings.Split(imageName, "/")
	if len(parts) < 3 {
		return false, fmt.Errorf("formato de imagem GHCR inválido: %s", imageName)
	}

	repositoryName := strings.Join(parts[2:], "/")
	if strings.Contains(repositoryName, ":") {
		repositoryName = strings.Split(repositoryName, ":")[0]
	}

	imageTag := "latest"
	if strings.Contains(imageName, ":") {
		imageTag = strings.Split(imageName, ":")[1]
	}

	url := fmt.Sprintf("https://ghcr.io/v2/%s/%s/manifests/%s", r.Organization, repositoryName, imageTag)

	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return false, err
	}

	if r.Username != "" && r.Password != "" {
		req.SetBasicAuth(r.Username, r.Password)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}
