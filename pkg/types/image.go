package types

import (
	"fmt"
	"strings"
)

type ImageInfo struct {
	Image           string `json:"image"`
	ResourceType    string `json:"resource_type"`
	ResourceName    string `json:"resource_name"`
	Namespace       string `json:"namespace"`
	Container       string `json:"container"`
	IsInitContainer bool   `json:"is_init_container"`
	IsPublic        bool   `json:"is_public"`
	Registry        string `json:"registry,omitempty"`
	Repository      string `json:"repository,omitempty"`
	Tag             string `json:"tag,omitempty"`
}

type ParsedImage struct {
	OriginalImage  string
	Registry       string
	Namespace      string
	Repository     string
	FullRepository string
	Tag            string
	Digest         string
}

func ParseImageName(imageName string) *ParsedImage {
	parsed := &ParsedImage{
		OriginalImage: imageName,
		Tag:           "latest",
	}

	workingImage := imageName

	if strings.Contains(workingImage, "@") {
		parts := strings.Split(workingImage, "@")
		workingImage = parts[0]
		parsed.Digest = parts[1]
	}

	if strings.Contains(workingImage, ":") {
		parts := strings.Split(workingImage, ":")
		workingImage = parts[0]
		parsed.Tag = parts[1]
	}

	parts := strings.Split(workingImage, "/")

	switch len(parts) {
	case 1:
		parsed.Registry = "docker.io"
		parsed.Namespace = "library"
		parsed.Repository = parts[0]
		parsed.FullRepository = fmt.Sprintf("library/%s", parts[0])

	case 2:
		if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
			parsed.Registry = parts[0]
			parsed.Namespace = ""
			parsed.Repository = parts[1]
			parsed.FullRepository = parts[1]
		} else {
			parsed.Registry = "docker.io"
			parsed.Namespace = parts[0]
			parsed.Repository = parts[1]
			parsed.FullRepository = fmt.Sprintf("%s/%s", parts[0], parts[1])
		}

	case 3:
		parsed.Registry = parts[0]
		parsed.Namespace = parts[1]
		parsed.Repository = parts[2]
		parsed.FullRepository = fmt.Sprintf("%s/%s", parts[1], parts[2])

	default:
		parsed.Registry = parts[0]
		parsed.Repository = parts[len(parts)-1]
		parsed.Namespace = strings.Join(parts[1:len(parts)-1], "/")
		parsed.FullRepository = strings.Join(parts[1:], "/")
	}

	if parsed.Registry == "index.docker.io" || parsed.Registry == "registry-1.docker.io" {
		parsed.Registry = "docker.io"
	}

	return parsed
}
