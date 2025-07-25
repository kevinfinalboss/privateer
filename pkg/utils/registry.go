package utils

import (
	"fmt"
	"strings"
)

func IsPublicRegistry(registry string) bool {
	publicRegistries := []string{
		"docker.io",
		"registry.hub.docker.com",
		"quay.io",
		"gcr.io",
		"registry.k8s.io",
		"k8s.gcr.io",
		"ghcr.io",
		"public.ecr.aws",
		"mcr.microsoft.com",
		"index.docker.io",
		"registry-1.docker.io",
	}

	for _, pubReg := range publicRegistries {
		if registry == pubReg {
			return true
		}
	}

	return false
}

func ExtractRegistry(imageName string) string {
	if strings.Contains(imageName, "/") {
		parts := strings.Split(imageName, "/")
		if len(parts) >= 2 && strings.Contains(parts[0], ".") {
			return parts[0]
		}
	}
	return "docker.io"
}

func ExtractRepository(imageName string) string {
	if strings.Contains(imageName, ":") {
		return strings.Split(imageName, ":")[0]
	}
	return imageName
}

func ExtractRepositoryOnly(imageName string) string {
	fullRepo := ExtractRepository(imageName)
	parts := strings.Split(fullRepo, "/")
	return parts[len(parts)-1]
}

func ExtractTag(imageName string) string {
	if strings.Contains(imageName, ":") {
		parts := strings.Split(imageName, ":")
		return parts[len(parts)-1]
	}
	return "latest"
}

func BuildFullImageName(registry, repository, tag string) string {
	if registry == "" || registry == "docker.io" {
		return fmt.Sprintf("%s:%s", repository, tag)
	}
	return fmt.Sprintf("%s/%s:%s", registry, repository, tag)
}

func BuildDockerIOImageName(repository, tag string) string {
	if !strings.Contains(repository, "/") {

		return fmt.Sprintf("docker.io/library/%s:%s", repository, tag)
	}
	return fmt.Sprintf("docker.io/%s:%s", repository, tag)
}
