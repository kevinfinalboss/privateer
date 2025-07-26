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

type GitOpsResult struct {
	Repository     string             `json:"repository"`
	Branch         string             `json:"branch"`
	PullRequest    *PullRequestInfo   `json:"pull_request,omitempty"`
	FilesChanged   []FileChange       `json:"files_changed"`
	ImagesChanged  []ImageReplacement `json:"images_changed"`
	Success        bool               `json:"success"`
	Error          error              `json:"error,omitempty"`
	ProcessingTime string             `json:"processing_time"`
}

type FileChange struct {
	FilePath      string             `json:"file_path"`
	FileType      string             `json:"file_type"`
	Changes       []ImageReplacement `json:"changes"`
	LinesChanged  int                `json:"lines_changed"`
	Validated     bool               `json:"validated"`
	BackupContent string             `json:"backup_content,omitempty"`
}

type ImageReplacement struct {
	SourceImage    string `json:"source_image"`
	TargetImage    string `json:"target_image"`
	FileType       string `json:"file_type"`
	FilePath       string `json:"file_path"`
	LineNumber     int    `json:"line_number"`
	Context        string `json:"context"`
	ReplacementKey string `json:"replacement_key"`
}

type PullRequestInfo struct {
	URL       string   `json:"url"`
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Reviewers []string `json:"reviewers"`
	Labels    []string `json:"labels"`
	Draft     bool     `json:"draft"`
	Mergeable bool     `json:"mergeable"`
	State     string   `json:"state"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

type GitOpsSummary struct {
	TotalRepositories     int             `json:"total_repositories"`
	ProcessedRepositories int             `json:"processed_repositories"`
	SuccessfulPRs         int             `json:"successful_prs"`
	FailedOperations      int             `json:"failed_operations"`
	TotalFilesChanged     int             `json:"total_files_changed"`
	TotalImagesReplaced   int             `json:"total_images_replaced"`
	Results               []*GitOpsResult `json:"results"`
	ProcessingTime        string          `json:"processing_time"`
	Errors                []error         `json:"errors,omitempty"`
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

type BranchOperation struct {
	Repository   string `json:"repository"`
	BaseBranch   string `json:"base_branch"`
	TargetBranch string `json:"target_branch"`
	Created      bool   `json:"created"`
	Exists       bool   `json:"exists"`
	CommitSHA    string `json:"commit_sha"`
}

type GitOpsValidation struct {
	FilePath     string   `json:"file_path"`
	IsValid      bool     `json:"is_valid"`
	SyntaxErrors []string `json:"syntax_errors,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
	Suggestions  []string `json:"suggestions,omitempty"`
}

type SearchPattern struct {
	Pattern     string   `yaml:"pattern"`
	FileTypes   []string `yaml:"file_types"`
	Description string   `yaml:"description"`
	Enabled     bool     `yaml:"enabled"`
}

type GitOpsConfig struct {
	Enabled         bool                `yaml:"enabled"`
	Strategy        string              `yaml:"strategy"`
	AutoPR          bool                `yaml:"auto_pr"`
	BranchPrefix    string              `yaml:"branch_prefix"`
	CommitMessage   string              `yaml:"commit_message"`
	SearchPatterns  []SearchPattern     `yaml:"search_patterns"`
	MappingRules    []RepositoryMapping `yaml:"mapping_rules"`
	ValidationRules ValidationConfig    `yaml:"validation"`
	TagResolution   TagResolutionConfig `yaml:"tag_resolution"`
}

type ValidationConfig struct {
	ValidateYAML     bool `yaml:"validate_yaml"`
	ValidateHelm     bool `yaml:"validate_helm"`
	ValidateBrackets bool `yaml:"validate_brackets"`
	CheckImageExists bool `yaml:"check_image_exists"`
	DryRunKubernetes bool `yaml:"dry_run_kubernetes"`
}
