package registry

import (
	"context"
	"fmt"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

type GHCRRegistry struct {
	*BaseRegistry
	Organization string
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

	return &GHCRRegistry{
		BaseRegistry: base,
	}, nil
}

func (r *GHCRRegistry) Login(ctx context.Context) error {
	r.Logger.Info("ghcr_not_implemented").
		Str("registry", r.Name).
		Send()

	return fmt.Errorf("GHCR ainda não implementado - será adicionado na próxima versão")
}

func (r *GHCRRegistry) Pull(ctx context.Context, imageName string) error {
	return fmt.Errorf("GHCR ainda não implementado")
}

func (r *GHCRRegistry) Push(ctx context.Context, image *types.ImageInfo, targetTag string) error {
	return fmt.Errorf("GHCR ainda não implementado")
}

func (r *GHCRRegistry) Copy(ctx context.Context, sourceImage, targetImage string) error {
	return fmt.Errorf("GHCR ainda não implementado")
}

func (r *GHCRRegistry) IsHealthy(ctx context.Context) error {
	return fmt.Errorf("GHCR ainda não implementado")
}
