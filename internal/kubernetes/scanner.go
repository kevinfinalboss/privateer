package kubernetes

import (
	"context"
	"strings"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Scanner struct {
	client *Client
	logger *logger.Logger
	config *types.Config
}

func NewScanner(client *Client, log *logger.Logger, cfg *types.Config) *Scanner {
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

	s.logger.Debug("namespace_scan_summary_before_filtering").
		Str("namespace", namespace).
		Int("total_images_found", len(allImages)).
		Send()

	for i, img := range allImages {
		s.logger.Debug("image_found_before_filtering").
			Int("index", i).
			Str("namespace", namespace).
			Str("image", img.Image).
			Str("resource_type", img.ResourceType).
			Str("resource_name", img.ResourceName).
			Str("container", img.Container).
			Bool("is_init_container", img.IsInitContainer).
			Send()
	}

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

	s.logger.Debug("starting_image_filtering").
		Int("total_images", len(images)).
		Send()

	for i, image := range images {
		s.logger.Debug("analyzing_image_publicity").
			Int("index", i).
			Str("image", image.Image).
			Str("namespace", image.Namespace).
			Str("resource", image.ResourceName).
			Send()

		isPublic := s.isPublicImage(image.Image)

		s.logger.Debug("image_publicity_result").
			Str("image", image.Image).
			Str("namespace", image.Namespace).
			Bool("is_public", isPublic).
			Send()

		if isPublic {
			image.IsPublic = true
			publicImages = append(publicImages, image)

			s.logger.Debug("image_added_to_public_list").
				Str("image", image.Image).
				Str("namespace", image.Namespace).
				Int("current_public_count", len(publicImages)).
				Send()
		} else {
			s.logger.Debug("image_excluded_from_public_list").
				Str("image", image.Image).
				Str("namespace", image.Namespace).
				Str("reason", "classified_as_private").
				Send()
		}
	}

	s.logger.Debug("image_filtering_completed").
		Int("total_analyzed", len(images)).
		Int("public_images_found", len(publicImages)).
		Send()

	return publicImages
}

func (s *Scanner) isPublicImage(imageName string) bool {
	imageLower := strings.ToLower(imageName)

	s.logger.Debug("starting_image_classification").
		Str("image", imageName).
		Str("image_lower", imageLower).
		Send()

	if s.shouldIgnoreRegistry(imageName) {
		s.logger.Debug("image_classification_result").
			Str("image", imageName).
			Str("decision", "ignored").
			Str("reason", "registry_in_ignore_list").
			Bool("is_public", false).
			Send()
		return false
	}

	if s.isCustomPrivateRegistry(imageName) {
		s.logger.Debug("image_classification_result").
			Str("image", imageName).
			Str("decision", "private").
			Str("reason", "custom_private_registry").
			Bool("is_public", false).
			Send()
		return false
	}

	if s.isCustomPublicRegistry(imageName) {
		s.logger.Debug("image_classification_result").
			Str("image", imageName).
			Str("decision", "public").
			Str("reason", "custom_public_registry").
			Bool("is_public", true).
			Send()
		return true
	}

	knownPrivateRegistries := []string{
		"localhost",
		"127.0.0.1",
	}

	for _, registry := range knownPrivateRegistries {
		if strings.HasPrefix(imageLower, registry) {
			s.logger.Debug("image_classification_result").
				Str("image", imageName).
				Str("decision", "private").
				Str("reason", "known_private_registry").
				Str("matched_registry", registry).
				Bool("is_public", false).
				Send()
			return false
		}
	}

	if s.isPrivateRegistry(imageName) {
		s.logger.Debug("image_classification_result").
			Str("image", imageName).
			Str("decision", "private").
			Str("reason", "detected_as_private_registry").
			Bool("is_public", false).
			Send()
		return false
	}

	s.logger.Debug("image_classification_result").
		Str("image", imageName).
		Str("decision", "public").
		Str("reason", "default_public_classification").
		Bool("is_public", true).
		Send()
	return true
}

func (s *Scanner) shouldIgnoreRegistry(imageName string) bool {
	if s.config == nil || len(s.config.ImageDetection.IgnoreRegistries) == 0 {
		s.logger.Debug("ignore_registry_check").
			Str("image", imageName).
			Bool("has_config", s.config != nil).
			Int("ignore_list_size", 0).
			Bool("should_ignore", false).
			Send()
		return false
	}

	imageLower := strings.ToLower(imageName)
	for _, ignored := range s.config.ImageDetection.IgnoreRegistries {
		if strings.HasPrefix(imageLower, strings.ToLower(ignored)) {
			s.logger.Debug("ignore_registry_check").
				Str("image", imageName).
				Str("matched_ignore_pattern", ignored).
				Bool("should_ignore", true).
				Send()
			return true
		}
	}

	s.logger.Debug("ignore_registry_check").
		Str("image", imageName).
		Int("ignore_list_size", len(s.config.ImageDetection.IgnoreRegistries)).
		Bool("should_ignore", false).
		Send()
	return false
}

func (s *Scanner) isCustomPrivateRegistry(imageName string) bool {
	if s.config == nil || len(s.config.ImageDetection.CustomPrivateRegistries) == 0 {
		s.logger.Debug("custom_private_registry_check").
			Str("image", imageName).
			Bool("has_config", s.config != nil).
			Int("private_list_size", 0).
			Bool("is_custom_private", false).
			Send()
		return false
	}

	imageLower := strings.ToLower(imageName)
	for _, privateReg := range s.config.ImageDetection.CustomPrivateRegistries {
		if strings.HasPrefix(imageLower, strings.ToLower(privateReg)) {
			s.logger.Debug("custom_private_registry_check").
				Str("image", imageName).
				Str("matched_private_pattern", privateReg).
				Bool("is_custom_private", true).
				Send()
			return true
		}
	}

	s.logger.Debug("custom_private_registry_check").
		Str("image", imageName).
		Int("private_list_size", len(s.config.ImageDetection.CustomPrivateRegistries)).
		Bool("is_custom_private", false).
		Send()
	return false
}

func (s *Scanner) isCustomPublicRegistry(imageName string) bool {
	if s.config == nil || len(s.config.ImageDetection.CustomPublicRegistries) == 0 {
		s.logger.Debug("custom_public_registry_check").
			Str("image", imageName).
			Bool("has_config", s.config != nil).
			Int("public_list_size", 0).
			Bool("is_custom_public", false).
			Send()
		return false
	}

	imageLower := strings.ToLower(imageName)
	for _, publicReg := range s.config.ImageDetection.CustomPublicRegistries {
		if strings.HasPrefix(imageLower, strings.ToLower(publicReg)) {
			s.logger.Debug("custom_public_registry_check").
				Str("image", imageName).
				Str("matched_public_pattern", publicReg).
				Bool("is_custom_public", true).
				Send()
			return true
		}
	}

	s.logger.Debug("custom_public_registry_check").
		Str("image", imageName).
		Int("public_list_size", len(s.config.ImageDetection.CustomPublicRegistries)).
		Bool("is_custom_public", false).
		Send()
	return false
}

func (s *Scanner) isPrivateRegistry(imageName string) bool {
	imageLower := strings.ToLower(imageName)

	s.logger.Debug("private_registry_detection_start").
		Str("image", imageName).
		Send()

	if strings.Contains(imageLower, ".dkr.ecr.") &&
		strings.Contains(imageLower, ".amazonaws.com") &&
		!strings.HasPrefix(imageLower, "public.ecr.aws") {
		s.logger.Debug("private_registry_detection").
			Str("image", imageName).
			Str("type", "ecr_private").
			Bool("is_private", true).
			Send()
		return true
	}

	if strings.Contains(imageLower, ".azurecr.io") &&
		!strings.Contains(imageLower, "mcr.microsoft.com") {
		s.logger.Debug("private_registry_detection").
			Str("image", imageName).
			Str("type", "azure_private").
			Bool("is_private", true).
			Send()
		return true
	}

	if (strings.Contains(imageLower, ".gcr.io") ||
		strings.Contains(imageLower, ".pkg.dev")) &&
		!strings.HasPrefix(imageLower, "gcr.io/google-containers") &&
		!strings.HasPrefix(imageLower, "k8s.gcr.io") &&
		!strings.HasPrefix(imageLower, "registry.k8s.io") {
		s.logger.Debug("private_registry_detection").
			Str("image", imageName).
			Str("type", "gcp_private").
			Bool("is_private", true).
			Send()
		return true
	}

	if strings.HasPrefix(imageLower, "ghcr.io/") {
		parts := strings.Split(imageName, "/")
		if len(parts) >= 3 {
			s.logger.Debug("private_registry_detection").
				Str("image", imageName).
				Str("type", "ghcr_private").
				Int("parts_count", len(parts)).
				Bool("is_private", true).
				Send()
			return true
		}
	}

	parts := strings.Split(imageName, "/")
	if len(parts) >= 2 && strings.Contains(parts[0], ".") &&
		!strings.Contains(parts[0], "docker.io") &&
		!strings.Contains(parts[0], "index.docker.io") &&
		!strings.Contains(parts[0], "registry-1.docker.io") {
		s.logger.Debug("private_registry_detection").
			Str("image", imageName).
			Str("type", "custom_domain_private").
			Str("detected_registry", parts[0]).
			Int("parts_count", len(parts)).
			Bool("is_private", true).
			Send()
		return true
	}

	s.logger.Debug("private_registry_detection").
		Str("image", imageName).
		Str("type", "not_detected_as_private").
		Bool("is_private", false).
		Send()
	return false
}
