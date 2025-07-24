package types

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
