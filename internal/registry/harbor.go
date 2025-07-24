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

type HarborRegistry struct {
	*BaseRegistry
	httpClient *http.Client
	Project    string
}

func NewHarborRegistry(config *types.RegistryConfig, logger *logger.Logger) (*HarborRegistry, error) {
	base := &BaseRegistry{
		Name:     config.Name,
		Type:     "harbor",
		Logger:   logger,
		Username: config.Username,
		Password: config.Password,
		URL:      config.URL,
		Insecure: config.Insecure,
	}

	httpClient := createHTTPClient(config.Insecure)

	return &HarborRegistry{
		BaseRegistry: base,
		httpClient:   httpClient,
		Project:      "library",
	}, nil
}

func (r *HarborRegistry) Login(ctx context.Context) error {
	r.Logger.Debug("registry_login_start").
		Str("registry", r.Name).
		Str("url", r.URL).
		Send()

	cmd := exec.CommandContext(ctx, "docker", "login", r.URL, "-u", r.Username, "--password-stdin")
	cmd.Stdin = strings.NewReader(r.Password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("registry_login_failed").
			Str("registry", r.Name).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha no login do Harbor %s: %w", r.Name, err)
	}

	r.Logger.Info("registry_login_success").
		Str("registry", r.Name).
		Send()

	return nil
}

func (r *HarborRegistry) Pull(ctx context.Context, imageName string) error {
	r.Logger.Debug("image_pull_start").
		Str("image", imageName).
		Send()

	cmd := exec.CommandContext(ctx, "docker", "pull", imageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("image_pull_failed").
			Str("image", imageName).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer pull da imagem %s: %w", imageName, err)
	}

	r.Logger.Info("image_pull_success").
		Str("image", imageName).
		Send()

	return nil
}

func (r *HarborRegistry) Push(ctx context.Context, image *types.ImageInfo, targetTag string) error {
	r.Logger.Debug("image_push_start").
		Str("source", image.Image).
		Str("target", targetTag).
		Send()

	cmd := exec.CommandContext(ctx, "docker", "tag", image.Image, targetTag)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("image_tag_failed").
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
		r.Logger.Error("image_push_failed").
			Str("target", targetTag).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer push da imagem %s: %w", targetTag, err)
	}

	r.Logger.Info("image_push_success").
		Str("target", targetTag).
		Send()

	return nil
}

func (r *HarborRegistry) Copy(ctx context.Context, sourceImage, targetImage string) error {
	r.Logger.Debug("image_copy_start").
		Str("source", sourceImage).
		Str("target", targetImage).
		Send()

	if err := r.Pull(ctx, sourceImage); err != nil {
		return err
	}

	targetWithProject := fmt.Sprintf("%s/%s/%s", r.URL, r.Project,
		strings.TrimPrefix(targetImage, r.URL+"/"))

	cmd := exec.CommandContext(ctx, "docker", "tag", sourceImage, targetWithProject)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("image_tag_failed").
			Str("source", sourceImage).
			Str("target", targetWithProject).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer tag da imagem: %w", err)
	}

	cmd = exec.CommandContext(ctx, "docker", "push", targetWithProject)
	output, err = cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("image_push_failed").
			Str("target", targetWithProject).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer push da imagem %s: %w", targetWithProject, err)
	}

	r.Logger.Info("image_copy_success").
		Str("source", sourceImage).
		Str("target", targetWithProject).
		Send()

	return nil
}

func (r *HarborRegistry) IsHealthy(ctx context.Context) error {
	var baseURL string
	if strings.HasPrefix(r.URL, "http://") || strings.HasPrefix(r.URL, "https://") {
		baseURL = strings.TrimSuffix(r.URL, "/")
	} else {
		if r.Insecure {
			baseURL = fmt.Sprintf("http://%s", r.URL)
		} else {
			baseURL = fmt.Sprintf("https://%s", r.URL)
		}
	}

	url := fmt.Sprintf("%s/api/v2.0/health", baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("falha na conexão com Harbor: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Harbor retornou status %d", resp.StatusCode)
	}

	return nil
}
