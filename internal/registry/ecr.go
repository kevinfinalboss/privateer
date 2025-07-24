package registry

import (
	"context"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrTypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

type ECRRegistry struct {
	*BaseRegistry
	Region    string
	AccountID string
	Profiles  []string
	AccessKey string
	SecretKey string
	awsConfig aws.Config
	ecrClient *ecr.Client
}

func NewECRRegistry(config *types.RegistryConfig, logger *logger.Logger) (*ECRRegistry, error) {
	base := &BaseRegistry{
		Name:   config.Name,
		Type:   "ecr",
		Logger: logger,
	}

	registry := &ECRRegistry{
		BaseRegistry: base,
		Region:       config.Region,
		AccountID:    config.AccountID,
		Profiles:     config.Profiles,
		AccessKey:    config.AccessKey,
		SecretKey:    config.SecretKey,
	}

	if err := registry.initAWSConfig(context.Background()); err != nil {
		return nil, fmt.Errorf("falha ao inicializar configuração AWS: %w", err)
	}

	return registry, nil
}

func (r *ECRRegistry) initAWSConfig(ctx context.Context) error {
	var cfg aws.Config
	var err error

	if r.AccessKey != "" && r.SecretKey != "" {
		r.Logger.Debug("ecr_using_credentials").
			Str("access_key", r.AccessKey[:8]+"...").
			Send()
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(r.Region),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				r.AccessKey,
				r.SecretKey,
				"",
			)),
		)
	} else if len(r.Profiles) > 0 {
		r.Logger.Debug("ecr_using_profiles").
			Strs("profiles", r.Profiles).
			Send()

		for i, profile := range r.Profiles {
			r.Logger.Debug("ecr_trying_profile").
				Str("profile", profile).
				Int("attempt", i+1).
				Send()

			cfg, err = config.LoadDefaultConfig(ctx,
				config.WithRegion(r.Region),
				config.WithSharedConfigProfile(profile),
			)

			if err == nil {
				r.Logger.Info("ecr_profile_success").
					Str("profile", profile).
					Send()
				break
			}

			r.Logger.Warn("ecr_profile_failed").
				Str("profile", profile).
				Err(err).
				Send()
		}

		if err != nil {
			return fmt.Errorf("falha em todos os profiles AWS: %w", err)
		}
	} else {
		r.Logger.Debug("ecr_using_default_credentials").Send()
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(r.Region),
		)
	}

	if err != nil {
		return fmt.Errorf("falha ao carregar configuração AWS: %w", err)
	}

	r.awsConfig = cfg
	r.ecrClient = ecr.NewFromConfig(cfg)

	if r.AccountID == "" {
		if err := r.discoverAccountID(ctx); err != nil {
			return fmt.Errorf("falha ao descobrir Account ID: %w", err)
		}
	}

	return nil
}

func (r *ECRRegistry) discoverAccountID(ctx context.Context) error {
	stsClient := sts.NewFromConfig(r.awsConfig)

	result, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return err
	}

	r.AccountID = *result.Account
	r.Logger.Debug("account_id_discovered").
		Str("account_id", r.AccountID).
		Send()

	return nil
}

func (r *ECRRegistry) Login(ctx context.Context) error {
	r.Logger.Debug("ecr_login_start").
		Str("registry", r.Name).
		Str("region", r.Region).
		Str("account_id", r.AccountID).
		Send()

	result, err := r.ecrClient.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		r.Logger.Error("ecr_auth_failed").
			Str("registry", r.Name).
			Err(err).
			Send()
		return fmt.Errorf("falha na autenticação ECR: %w", err)
	}

	if len(result.AuthorizationData) == 0 {
		return fmt.Errorf("nenhum token de autorização retornado pelo ECR")
	}

	authData := result.AuthorizationData[0]
	token, err := base64.StdEncoding.DecodeString(*authData.AuthorizationToken)
	if err != nil {
		return fmt.Errorf("falha ao decodificar token ECR: %w", err)
	}

	parts := strings.SplitN(string(token), ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("formato de token ECR inválido")
	}

	registryURL := *authData.ProxyEndpoint
	username := parts[0]
	password := parts[1]

	cmd := exec.CommandContext(ctx, "docker", "login", registryURL, "-u", username, "--password-stdin")
	cmd.Stdin = strings.NewReader(password)

	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("ecr_docker_login_failed").
			Str("registry", r.Name).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha no login Docker para ECR: %w", err)
	}

	r.Logger.Info("ecr_login_success").
		Str("registry", r.Name).
		Str("endpoint", registryURL).
		Send()

	return nil
}

func (r *ECRRegistry) Pull(ctx context.Context, imageName string) error {
	r.Logger.Debug("ecr_pull_start").
		Str("image", imageName).
		Send()

	cmd := exec.CommandContext(ctx, "docker", "pull", imageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("ecr_pull_failed").
			Str("image", imageName).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer pull da imagem %s: %w", imageName, err)
	}

	r.Logger.Info("ecr_pull_success").
		Str("image", imageName).
		Send()

	return nil
}

func (r *ECRRegistry) Push(ctx context.Context, image *types.ImageInfo, targetTag string) error {
	r.Logger.Debug("ecr_push_start").
		Str("source", image.Image).
		Str("target", targetTag).
		Send()

	repositoryName := r.extractRepositoryName(targetTag)
	if err := r.ensureRepositoryExists(ctx, repositoryName); err != nil {
		r.Logger.Warn("ecr_repository_create_failed").
			Str("repository", repositoryName).
			Err(err).
			Send()
	}

	cmd := exec.CommandContext(ctx, "docker", "tag", image.Image, targetTag)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("ecr_tag_failed").
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
		r.Logger.Error("ecr_push_failed").
			Str("target", targetTag).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer push da imagem %s: %w", targetTag, err)
	}

	r.Logger.Info("ecr_push_success").
		Str("target", targetTag).
		Send()

	return nil
}

func (r *ECRRegistry) Copy(ctx context.Context, sourceImage, targetImage string) error {
	r.Logger.Debug("ecr_copy_start").
		Str("source", sourceImage).
		Str("target", targetImage).
		Send()

	repositoryName := r.extractRepositoryName(targetImage)
	if err := r.ensureRepositoryExists(ctx, repositoryName); err != nil {
		r.Logger.Warn("ecr_repository_create_failed").
			Str("repository", repositoryName).
			Err(err).
			Send()
	}

	if err := r.Pull(ctx, sourceImage); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "docker", "tag", sourceImage, targetImage)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Logger.Error("ecr_tag_failed").
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
		r.Logger.Error("ecr_push_failed").
			Str("target", targetImage).
			Str("output", string(output)).
			Err(err).
			Send()
		return fmt.Errorf("falha ao fazer push da imagem %s: %w", targetImage, err)
	}

	r.Logger.Info("ecr_copy_success").
		Str("source", sourceImage).
		Str("target", targetImage).
		Send()

	return nil
}

func (r *ECRRegistry) IsHealthy(ctx context.Context) error {
	r.Logger.Debug("ecr_health_check").
		Str("registry", r.Name).
		Send()

	_, err := r.ecrClient.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return fmt.Errorf("falha no health check ECR: %w", err)
	}

	return nil
}

func (r *ECRRegistry) HasImage(ctx context.Context, imageName string) (bool, error) {
	repositoryName := r.extractRepositoryName(imageName)
	imageTag := r.extractImageTag(imageName)

	r.Logger.Debug("ecr_checking_image").
		Str("repository", repositoryName).
		Str("tag", imageTag).
		Send()

	_, err := r.ecrClient.BatchGetImage(ctx, &ecr.BatchGetImageInput{
		RepositoryName: aws.String(repositoryName),
		ImageIds: []ecrTypes.ImageIdentifier{
			{
				ImageTag: aws.String(imageTag),
			},
		},
	})

	if err != nil {
		if strings.Contains(err.Error(), "does not exist") ||
			strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "RepositoryNotFound") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (r *ECRRegistry) ensureRepositoryExists(ctx context.Context, repositoryName string) error {
	r.Logger.Debug("ecr_checking_repository").
		Str("repository", repositoryName).
		Send()

	_, err := r.ecrClient.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{
		RepositoryNames: []string{repositoryName},
	})

	if err != nil {
		if strings.Contains(err.Error(), "RepositoryNotFound") {
			return r.createRepository(ctx, repositoryName)
		}
		return err
	}

	r.Logger.Debug("ecr_repository_exists").
		Str("repository", repositoryName).
		Send()

	return nil
}

func (r *ECRRegistry) createRepository(ctx context.Context, repositoryName string) error {
	r.Logger.Debug("ecr_creating_repository").
		Str("repository", repositoryName).
		Send()

	_, err := r.ecrClient.CreateRepository(ctx, &ecr.CreateRepositoryInput{
		RepositoryName: aws.String(repositoryName),
	})

	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			r.Logger.Debug("ecr_repository_exists").
				Str("repository", repositoryName).
				Send()
			return nil
		}
		return fmt.Errorf("falha ao criar repositório ECR %s: %w", repositoryName, err)
	}

	r.Logger.Info("ecr_repository_created").
		Str("repository", repositoryName).
		Send()

	return nil
}

func (r *ECRRegistry) GetRegistryURL() string {
	return fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", r.AccountID, r.Region)
}

func (r *ECRRegistry) extractRepositoryName(imageName string) string {
	parts := strings.Split(imageName, "/")
	if len(parts) < 2 {
		return imageName
	}

	repositoryName := strings.Join(parts[1:], "/")
	if strings.Contains(repositoryName, ":") {
		repositoryName = strings.Split(repositoryName, ":")[0]
	}

	return repositoryName
}

func (r *ECRRegistry) extractImageTag(imageName string) string {
	if strings.Contains(imageName, ":") {
		parts := strings.Split(imageName, ":")
		return parts[len(parts)-1]
	}
	return "latest"
}
