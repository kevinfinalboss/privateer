package cli

import (
	"github.com/kevinfinalboss/privateer/internal/kubernetes"
	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Escaneia recursos procurando por imagens públicas",
	Long:  "Escaneia clusters Kubernetes ou repositórios GitHub procurando por imagens Docker públicas",
}

var scanClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Escaneia cluster Kubernetes",
	Long:  "Escaneia cluster Kubernetes procurando por imagens Docker públicas em Deployments, StatefulSets, DaemonSets, etc.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return scanCluster()
	},
}

var scanGithubCmd = &cobra.Command{
	Use:   "github",
	Short: "Escaneia repositórios GitHub",
	Long:  "Escaneia repositórios GitHub procurando por imagens Docker públicas em Dockerfiles e manifests Kubernetes",
	RunE: func(cmd *cobra.Command, args []string) error {
		return scanGithub()
	},
}

func init() {
	scanCmd.AddCommand(scanClusterCmd)
	scanCmd.AddCommand(scanGithubCmd)
}

func scanCluster() error {
	client, err := kubernetes.NewClient(cfg, log)
	if err != nil {
		return err
	}

	namespaces, err := client.GetNamespaces()
	if err != nil {
		log.Error("operation_failed").Err(err).Send()
		return err
	}

	log.Info("scanning_cluster").
		Int("namespace_count", len(namespaces)).
		Strs("namespaces", namespaces).
		Send()

	scanner := kubernetes.NewScanner(client, log, cfg)
	totalPublicImages := 0

	for _, namespace := range namespaces {
		publicImages, err := scanner.ScanNamespace(namespace)
		if err != nil {
			log.Error("operation_failed").
				Str("namespace", namespace).
				Err(err).
				Send()
			continue
		}

		if len(publicImages) > 0 {
			totalPublicImages += len(publicImages)

			for _, image := range publicImages {
				log.Info("public_image_found").
					Str("image", image.Image).
					Str("resource_type", image.ResourceType).
					Str("resource_name", image.ResourceName).
					Str("namespace", image.Namespace).
					Str("container", image.Container).
					Bool("is_init_container", image.IsInitContainer).
					Send()
			}
		}
	}

	log.Info("operation_completed").
		Str("operation", "cluster_scan").
		Int("total_public_images", totalPublicImages).
		Send()

	return nil
}

func scanGithub() error {
	log.Info("scanning_cluster").
		Str("organization", cfg.GitHub.Organization).
		Send()

	log.Info("operation_completed").
		Str("operation", "github_scan").
		Send()

	return nil
}
