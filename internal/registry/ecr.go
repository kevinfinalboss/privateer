package registry

import (
	"context"
	"fmt"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

type ECRRegistry struct {
	*BaseRegistry
	Region    string
	AccountID string
}

func NewECRRegistry(config *types.RegistryConfig, logger *logger.Logger) (*ECRRegistry, error) {
	base := &BaseRegistry{
		Name:     config.Name,
		Type:     "ecr",
		Logger:   logger,
		Username: config.Username,
		Password: config.Password,
		URL:      config.URL,
		Insecure: false,
	}

	return &ECRRegistry{
		BaseRegistry: base,
		Region:       config.Region,
	}, nil
}

func (r *ECRRegistry) Login(ctx context.Context) error {
	r.Logger.Info("ecr_not_implemented").
		Str("registry", r.Name).
		Send()

	return fmt.Errorf("ECR ainda não implementado - será adicionado na próxima versão")
}

func (r *ECRRegistry) Pull(ctx context.Context, imageName string) error {
	return fmt.Errorf("ECR ainda não implementado")
}

func (r *ECRRegistry) Push(ctx context.Context, image *types.ImageInfo, targetTag string) error {
	return fmt.Errorf("ECR ainda não implementado")
}

func (r *ECRRegistry) Copy(ctx context.Context, sourceImage, targetImage string) error {
	return fmt.Errorf("ECR ainda não implementado")
}

func (r *ECRRegistry) IsHealthy(ctx context.Context) error {
	return fmt.Errorf("ECR ainda não implementado")
}
