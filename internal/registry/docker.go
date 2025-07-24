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

type DockerRegistry struct {
	*BaseRegistry
	httpClient *http.Client
}

func NewDockerRegistry(config *types.RegistryConfig, logger *logger.Logger) (*DockerRegistry, error) {
	base := &BaseRegistry{
		Name:     config.Name,
		Type:     "docker",
		Logger:   logger,
		Username: config.Username,
		Password: config.Password,
		URL:      config.URL,
		Insecure: config.Insecure,
	}

	httpClient := createHTTPClient(config.Insecure)

	return &DockerRegistry{
		BaseRegistry: base,
		httpClient:   httpClient,
	}, nil
}

func (r *DockerRegistry) Login(ctx context.Context) error {
	r.Logger.Debug("registry_login_start").
		Str("registry", r.Name).
		Str("url", r.URL).
		Send()

	var registryURL string
	if strings.HasPrefix(r.URL, "http://") || strings.HasPrefix(r.URL, "https://") {
		registryURL = r.URL
	} else {
		registryURL = r.URL
	}

	cmd := exec.CommandContext(ctx, "docker", "login", registryURL, "-u", r.Username, "--password-stdin")
	cmd.Stdin = strings.NewReader(r.Password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("registry_login_failed").
			Str("registry", r.Name).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha no login do registry %s: %w", r.Name, err)
	}

	r.Logger.Info("registry_login_success").
		Str("registry", r.Name).
		Send()

	return nil
}

func (r *DockerRegistry) Pull(ctx context.Context, imageName string) error {
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

func (r *DockerRegistry) Push(ctx context.Context, image *types.ImageInfo, targetTag string) error {
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

func (r *DockerRegistry) Copy(ctx context.Context, sourceImage, targetImage string) error {
	r.Logger.Debug("image_copy_start").
		Str("source", sourceImage).
		Str("target", targetImage).
		Send()

	if err := r.Pull(ctx, sourceImage); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "docker", "tag", sourceImage, targetImage)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("image_tag_failed").
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
		r.Logger.Error("image_push_failed").
			Str("target", targetImage).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer push da imagem %s: %w", targetImage, err)
	}

	r.Logger.Info("image_copy_success").
		Str("source", sourceImage).
		Str("target", targetImage).
		Send()

	return nil
}

func (r *DockerRegistry) IsHealthy(ctx context.Context) error {
	var url string
	if strings.HasPrefix(r.URL, "http://") || strings.HasPrefix(r.URL, "https://") {
		url = fmt.Sprintf("%s/v2/", r.URL)
	} else {
		if r.Insecure {
			url = fmt.Sprintf("http://%s/v2/", r.URL)
		} else {
			url = fmt.Sprintf("https://%s/v2/", r.URL)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	if r.Username != "" && r.Password != "" {
		req.SetBasicAuth(r.Username, r.Password)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("falha na conexão com registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("registry retornou status %d", resp.StatusCode)
	}

	return nil
}
