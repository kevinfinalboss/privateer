package kubernetes

import (
	"context"
	"strings"

	"github.com/kevinfinalboss/privateer/internal/config"
	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Scanner struct {
	client *Client
	logger *logger.Logger
	config *config.Config
}

func NewScanner(client *Client, log *logger.Logger, cfg *config.Config) *Scanner {
	return &Scanner{
		client: client,
		logger: log,
		config: cfg,
	}
}

func (s *Scanner) ScanNamespace(namespace string) ([]*types.ImageInfo, error) {
	ctx := context.Background()
	var allImages []*types.ImageInfo

	s.logger.Info("scanning_namespace").
		Str("namespace", namespace).
		Send()

	deploymentImages, err := s.scanDeployments(ctx, namespace)
	if err != nil {
		return nil, err
	}
	allImages = append(allImages, deploymentImages...)

	statefulSetImages, err := s.scanStatefulSets(ctx, namespace)
	if err != nil {
		return nil, err
	}
	allImages = append(allImages, statefulSetImages...)

	daemonSetImages, err := s.scanDaemonSets(ctx, namespace)
	if err != nil {
		return nil, err
	}
	allImages = append(allImages, daemonSetImages...)

	jobImages, err := s.scanJobs(ctx, namespace)
	if err != nil {
		return nil, err
	}
	allImages = append(allImages, jobImages...)

	cronJobImages, err := s.scanCronJobs(ctx, namespace)
	if err != nil {
		return nil, err
	}
	allImages = append(allImages, cronJobImages...)

	publicImages := s.filterPublicImages(allImages)

	s.logger.Info("images_found").
		Str("namespace", namespace).
		Int("total_images", len(allImages)).
		Int("public_images", len(publicImages)).
		Send()

	return publicImages, nil
}

func (s *Scanner) scanDeployments(ctx context.Context, namespace string) ([]*types.ImageInfo, error) {
	deployments, err := s.client.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var images []*types.ImageInfo
	for _, deployment := range deployments.Items {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			imageInfo := &types.ImageInfo{
				Image:        container.Image,
				ResourceType: "Deployment",
				ResourceName: deployment.Name,
				Namespace:    namespace,
				Container:    container.Name,
			}
			images = append(images, imageInfo)
		}

		for _, container := range deployment.Spec.Template.Spec.InitContainers {
			imageInfo := &types.ImageInfo{
				Image:           container.Image,
				ResourceType:    "Deployment",
				ResourceName:    deployment.Name,
				Namespace:       namespace,
				Container:       container.Name,
				IsInitContainer: true,
			}
			images = append(images, imageInfo)
		}
	}

	s.logger.Debug("resource_scanned").
		Str("namespace", namespace).
		Str("resource_type", "Deployment").
		Int("resource_count", len(deployments.Items)).
		Int("image_count", len(images)).
		Send()

	return images, nil
}

func (s *Scanner) scanStatefulSets(ctx context.Context, namespace string) ([]*types.ImageInfo, error) {
	statefulSets, err := s.client.clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var images []*types.ImageInfo
	for _, statefulSet := range statefulSets.Items {
		for _, container := range statefulSet.Spec.Template.Spec.Containers {
			imageInfo := &types.ImageInfo{
				Image:        container.Image,
				ResourceType: "StatefulSet",
				ResourceName: statefulSet.Name,
				Namespace:    namespace,
				Container:    container.Name,
			}
			images = append(images, imageInfo)
		}

		for _, container := range statefulSet.Spec.Template.Spec.InitContainers {
			imageInfo := &types.ImageInfo{
				Image:           container.Image,
				ResourceType:    "StatefulSet",
				ResourceName:    statefulSet.Name,
				Namespace:       namespace,
				Container:       container.Name,
				IsInitContainer: true,
			}
			images = append(images, imageInfo)
		}
	}

	s.logger.Debug("resource_scanned").
		Str("namespace", namespace).
		Str("resource_type", "StatefulSet").
		Int("resource_count", len(statefulSets.Items)).
		Int("image_count", len(images)).
		Send()

	return images, nil
}

func (s *Scanner) scanDaemonSets(ctx context.Context, namespace string) ([]*types.ImageInfo, error) {
	daemonSets, err := s.client.clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var images []*types.ImageInfo
	for _, daemonSet := range daemonSets.Items {
		for _, container := range daemonSet.Spec.Template.Spec.Containers {
			imageInfo := &types.ImageInfo{
				Image:        container.Image,
				ResourceType: "DaemonSet",
				ResourceName: daemonSet.Name,
				Namespace:    namespace,
				Container:    container.Name,
			}
			images = append(images, imageInfo)
		}

		for _, container := range daemonSet.Spec.Template.Spec.InitContainers {
			imageInfo := &types.ImageInfo{
				Image:           container.Image,
				ResourceType:    "DaemonSet",
				ResourceName:    daemonSet.Name,
				Namespace:       namespace,
				Container:       container.Name,
				IsInitContainer: true,
			}
			images = append(images, imageInfo)
		}
	}

	s.logger.Debug("resource_scanned").
		Str("namespace", namespace).
		Str("resource_type", "DaemonSet").
		Int("resource_count", len(daemonSets.Items)).
		Int("image_count", len(images)).
		Send()

	return images, nil
}

func (s *Scanner) scanJobs(ctx context.Context, namespace string) ([]*types.ImageInfo, error) {
	jobs, err := s.client.clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var images []*types.ImageInfo
	for _, job := range jobs.Items {
		for _, container := range job.Spec.Template.Spec.Containers {
			imageInfo := &types.ImageInfo{
				Image:        container.Image,
				ResourceType: "Job",
				ResourceName: job.Name,
				Namespace:    namespace,
				Container:    container.Name,
			}
			images = append(images, imageInfo)
		}

		for _, container := range job.Spec.Template.Spec.InitContainers {
			imageInfo := &types.ImageInfo{
				Image:           container.Image,
				ResourceType:    "Job",
				ResourceName:    job.Name,
				Namespace:       namespace,
				Container:       container.Name,
				IsInitContainer: true,
			}
			images = append(images, imageInfo)
		}
	}

	s.logger.Debug("resource_scanned").
		Str("namespace", namespace).
		Str("resource_type", "Job").
		Int("resource_count", len(jobs.Items)).
		Int("image_count", len(images)).
		Send()

	return images, nil
}

func (s *Scanner) scanCronJobs(ctx context.Context, namespace string) ([]*types.ImageInfo, error) {
	cronJobs, err := s.client.clientset.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var images []*types.ImageInfo
	for _, cronJob := range cronJobs.Items {
		for _, container := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
			imageInfo := &types.ImageInfo{
				Image:        container.Image,
				ResourceType: "CronJob",
				ResourceName: cronJob.Name,
				Namespace:    namespace,
				Container:    container.Name,
			}
			images = append(images, imageInfo)
		}

		for _, container := range cronJob.Spec.JobTemplate.Spec.Template.Spec.InitContainers {
			imageInfo := &types.ImageInfo{
				Image:           container.Image,
				ResourceType:    "CronJob",
				ResourceName:    cronJob.Name,
				Namespace:       namespace,
				Container:       container.Name,
				IsInitContainer: true,
			}
			images = append(images, imageInfo)
		}
	}

	s.logger.Debug("resource_scanned").
		Str("namespace", namespace).
		Str("resource_type", "CronJob").
		Int("resource_count", len(cronJobs.Items)).
		Int("image_count", len(images)).
		Send()

	return images, nil
}

func (s *Scanner) filterPublicImages(images []*types.ImageInfo) []*types.ImageInfo {
	var publicImages []*types.ImageInfo

	for _, image := range images {
		if s.isPublicImage(image.Image) {
			image.IsPublic = true
			publicImages = append(publicImages, image)
		}
	}

	return publicImages
}

func (s *Scanner) isPublicImage(imageName string) bool {
	imageLower := strings.ToLower(imageName)

	if s.shouldIgnoreRegistry(imageName) {
		s.logger.Debug("registry_ignored").
			Str("image", imageName).
			Send()
		return false
	}

	if s.isCustomPrivateRegistry(imageName) {
		s.logger.Debug("custom_private_registry").
			Str("image", imageName).
			Send()
		return false
	}

	if s.isCustomPublicRegistry(imageName) {
		s.logger.Debug("custom_public_registry").
			Str("image", imageName).
			Send()
		return true
	}

	knownPrivateRegistries := []string{
		"localhost",
		"127.0.0.1",
	}

	for _, registry := range knownPrivateRegistries {
		if strings.HasPrefix(imageLower, registry) {
			return false
		}
	}

	if s.isPrivateRegistry(imageName) {
		return false
	}

	return true
}

func (s *Scanner) shouldIgnoreRegistry(imageName string) bool {
	if s.config == nil || len(s.config.ImageDetection.IgnoreRegistries) == 0 {
		return false
	}

	imageLower := strings.ToLower(imageName)
	for _, ignored := range s.config.ImageDetection.IgnoreRegistries {
		if strings.HasPrefix(imageLower, strings.ToLower(ignored)) {
			return true
		}
	}
	return false
}

func (s *Scanner) isCustomPrivateRegistry(imageName string) bool {
	if s.config == nil || len(s.config.ImageDetection.CustomPrivateRegistries) == 0 {
		return false
	}

	imageLower := strings.ToLower(imageName)
	for _, privateReg := range s.config.ImageDetection.CustomPrivateRegistries {
		if strings.HasPrefix(imageLower, strings.ToLower(privateReg)) {
			return true
		}
	}
	return false
}

func (s *Scanner) isCustomPublicRegistry(imageName string) bool {
	if s.config == nil || len(s.config.ImageDetection.CustomPublicRegistries) == 0 {
		return false
	}

	imageLower := strings.ToLower(imageName)
	for _, publicReg := range s.config.ImageDetection.CustomPublicRegistries {
		if strings.HasPrefix(imageLower, strings.ToLower(publicReg)) {
			return true
		}
	}
	return false
}

func (s *Scanner) isPrivateRegistry(imageName string) bool {
	imageLower := strings.ToLower(imageName)

	if strings.Contains(imageLower, ".dkr.ecr.") &&
		strings.Contains(imageLower, ".amazonaws.com") &&
		!strings.HasPrefix(imageLower, "public.ecr.aws") {
		return true
	}

	if strings.Contains(imageLower, ".azurecr.io") &&
		!strings.Contains(imageLower, "mcr.microsoft.com") {
		return true
	}

	if (strings.Contains(imageLower, ".gcr.io") ||
		strings.Contains(imageLower, ".pkg.dev")) &&
		!strings.HasPrefix(imageLower, "gcr.io/google-containers") &&
		!strings.HasPrefix(imageLower, "k8s.gcr.io") &&
		!strings.HasPrefix(imageLower, "registry.k8s.io") {
		return true
	}

	if strings.HasPrefix(imageLower, "ghcr.io/") {
		parts := strings.Split(imageName, "/")
		if len(parts) >= 3 {
			return true
		}
	}

	parts := strings.Split(imageName, "/")
	if len(parts) >= 2 && strings.Contains(parts[0], ".") &&
		!strings.Contains(parts[0], "docker.io") &&
		!strings.Contains(parts[0], "index.docker.io") &&
		!strings.Contains(parts[0], "registry-1.docker.io") {
		return true
	}

	return false
}
