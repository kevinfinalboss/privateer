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
	Project    string
	httpClient *http.Client
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

	project := config.Project
	if project == "" {
		project = "library"
	}

	httpClient := createHTTPClient(config.Insecure)

	return &HarborRegistry{
		BaseRegistry: base,
		Project:      project,
		httpClient:   httpClient,
	}, nil
}

func (r *HarborRegistry) Login(ctx context.Context) error {
	r.Logger.Debug("harbor_login_start").
		Str("registry", r.Name).
		Str("url", r.URL).
		Str("project", r.Project).
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
		r.Logger.Error("harbor_login_failed").
			Str("registry", r.Name).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha no login do Harbor %s: %w", r.Name, err)
	}

	r.Logger.Info("harbor_login_success").
		Str("registry", r.Name).
		Send()

	return nil
}

func (r *HarborRegistry) Pull(ctx context.Context, imageName string) error {
	r.Logger.Debug("harbor_pull_start").
		Str("image", imageName).
		Send()

	cmd := exec.CommandContext(ctx, "docker", "pull", imageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("harbor_pull_failed").
			Str("image", imageName).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer pull da imagem %s: %w", imageName, err)
	}

	r.Logger.Info("harbor_pull_success").
		Str("image", imageName).
		Send()

	return nil
}

func (r *HarborRegistry) Push(ctx context.Context, image *types.ImageInfo, targetTag string) error {
	r.Logger.Debug("harbor_push_start").
		Str("source", image.Image).
		Str("target", targetTag).
		Send()

	cmd := exec.CommandContext(ctx, "docker", "tag", image.Image, targetTag)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("harbor_tag_failed").
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
		r.Logger.Error("harbor_push_failed").
			Str("target", targetTag).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer push da imagem %s: %w", targetTag, err)
	}

	r.Logger.Info("harbor_push_success").
		Str("target", targetTag).
		Send()

	return nil
}

func (r *HarborRegistry) Copy(ctx context.Context, sourceImage, targetImage string) error {
	r.Logger.Debug("harbor_copy_start").
		Str("source", sourceImage).
		Str("target", targetImage).
		Send()

	if err := r.Pull(ctx, sourceImage); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "docker", "tag", sourceImage, targetImage)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("harbor_tag_failed").
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
		r.Logger.Error("harbor_push_failed").
			Str("target", targetImage).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer push da imagem %s: %w", targetImage, err)
	}

	r.Logger.Info("harbor_copy_success").
		Str("source", sourceImage).
		Str("target", targetImage).
		Send()

	return nil
}

func (r *HarborRegistry) IsHealthy(ctx context.Context) error {
	var url string
	if strings.HasPrefix(r.URL, "http://") || strings.HasPrefix(r.URL, "https://") {
		url = fmt.Sprintf("%s/api/v2.0/health", r.URL)
	} else {
		if r.Insecure {
			url = fmt.Sprintf("http://%s/api/v2.0/health", r.URL)
		} else {
			url = fmt.Sprintf("https://%s/api/v2.0/health", r.URL)
		}
	}

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

func (r *HarborRegistry) HasImage(ctx context.Context, imageName string) (bool, error) {
	parts := strings.Split(imageName, "/")
	if len(parts) < 2 {
		return false, fmt.Errorf("formato de imagem inválido: %s", imageName)
	}

	repositoryName := strings.Join(parts[1:], "/")
	if strings.Contains(repositoryName, ":") {
		repositoryName = strings.Split(repositoryName, ":")[0]
	}

	imageTag := "latest"
	if strings.Contains(imageName, ":") {
		imageTag = strings.Split(imageName, ":")[1]
	}

	var url string
	if strings.HasPrefix(r.URL, "http://") || strings.HasPrefix(r.URL, "https://") {
		url = fmt.Sprintf("%s/v2/%s/manifests/%s", r.URL, repositoryName, imageTag)
	} else {
		if r.Insecure {
			url = fmt.Sprintf("http://%s/v2/%s/manifests/%s", r.URL, repositoryName, imageTag)
		} else {
			url = fmt.Sprintf("https://%s/v2/%s/manifests/%s", r.URL, repositoryName, imageTag)
		}
	}

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
