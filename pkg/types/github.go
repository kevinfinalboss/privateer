package types

type GitHubRepositoryConfig struct {
	Name           string   `yaml:"name"`
	Enabled        bool     `yaml:"enabled"`
	Priority       int      `yaml:"priority"`
	Paths          []string `yaml:"paths"`
	ExcludedPaths  []string `yaml:"excluded_paths"`
	BranchStrategy string   `yaml:"branch_strategy"`
	PRSettings     PRConfig `yaml:"pr_settings"`
}

type PRConfig struct {
	AutoMerge    bool     `yaml:"auto_merge"`
	Reviewers    []string `yaml:"reviewers"`
	Labels       []string `yaml:"labels"`
	Template     string   `yaml:"template"`
	Draft        bool     `yaml:"draft"`
	CommitPrefix string   `yaml:"commit_prefix"`
}

type GitHubConfig struct {
	Enabled      bool                     `yaml:"enabled"`
	Token        string                   `yaml:"token"`
	Repositories []GitHubRepositoryConfig `yaml:"repositories"`
}

type FileTypeDetector struct {
	Extension string `json:"extension"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	Type      string `json:"type"`
}

type ImageDetectionResult struct {
	Image      string  `json:"image"`
	Repository string  `json:"repository,omitempty"`
	Tag        string  `json:"tag,omitempty"`
	Registry   string  `json:"registry,omitempty"`
	FullImage  string  `json:"full_image"`
	IsPublic   bool    `json:"is_public"`
	LineNumber int     `json:"line_number"`
	Context    string  `json:"context"`
	Confidence float64 `json:"confidence"`
	FilePath   string  `json:"file_path"`
}

type RepositoryMapping struct {
	Namespace   string  `yaml:"namespace"`
	AppName     string  `yaml:"app_name"`
	Repository  string  `yaml:"repository"`
	Path        string  `yaml:"path"`
	MappingType string  `yaml:"mapping_type"`
	Confidence  float64 `yaml:"confidence"`
	Source      string  `yaml:"source"`
}

type SearchPattern struct {
	Pattern     string   `yaml:"pattern"`
	FileTypes   []string `yaml:"file_types"`
	Description string   `yaml:"description"`
	Enabled     bool     `yaml:"enabled"`
}

type CreatePRRequest struct {
	Title               string `json:"title"`
	Head                string `json:"head"`
	Base                string `json:"base"`
	Body                string `json:"body"`
	MaintainerCanModify bool   `json:"maintainer_can_modify"`
	Draft               bool   `json:"draft"`
}

type PullRequestResponse struct {
	ID        int    `json:"id"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	State     string `json:"state"`
	HTMLURL   string `json:"html_url"`
	Mergeable *bool  `json:"mergeable"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

type ReviewerRequest struct {
	Reviewers     []string `json:"reviewers,omitempty"`
	TeamReviewers []string `json:"team_reviewers,omitempty"`
}

type LabelRequest struct {
	Labels []string `json:"labels"`
}

type GitHubResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}

type Repository struct {
	ID            int         `json:"id"`
	Name          string      `json:"name"`
	FullName      string      `json:"full_name"`
	Owner         Owner       `json:"owner"`
	Private       bool        `json:"private"`
	HTMLURL       string      `json:"html_url"`
	CloneURL      string      `json:"clone_url"`
	DefaultBranch string      `json:"default_branch"`
	Permissions   Permissions `json:"permissions,omitempty"`
}

type Owner struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

type Permissions struct {
	Admin bool `json:"admin"`
	Push  bool `json:"push"`
	Pull  bool `json:"pull"`
}

type Branch struct {
	Name      string `json:"name"`
	Commit    Commit `json:"commit"`
	Protected bool   `json:"protected"`
}

type Commit struct {
	SHA string `json:"sha"`
	URL string `json:"url"`
}

type FileContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	SHA         string `json:"sha"`
	Size        int    `json:"size"`
	URL         string `json:"url"`
	HTMLURL     string `json:"html_url"`
	GitURL      string `json:"git_url"`
	DownloadURL string `json:"download_url"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Encoding    string `json:"encoding"`
}

type TreeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	Size int    `json:"size,omitempty"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
}

type Tree struct {
	SHA  string      `json:"sha"`
	URL  string      `json:"url"`
	Tree []TreeEntry `json:"tree"`
}

type CreateBranchRequest struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

type UpdateFileRequest struct {
	Message   string     `json:"message"`
	Content   string     `json:"content"`
	SHA       string     `json:"sha,omitempty"`
	Branch    string     `json:"branch,omitempty"`
	Committer *Committer `json:"committer,omitempty"`
}

type Committer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UpdateFileResponse struct {
	Content struct {
		SHA  string `json:"sha"`
		Path string `json:"path"`
	} `json:"content"`
	Commit struct {
		SHA     string `json:"sha"`
		Message string `json:"message"`
	} `json:"commit"`
}
