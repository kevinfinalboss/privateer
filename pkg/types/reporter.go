package types

type ReportData struct {
	Title          string
	Timestamp      string
	ExecutionMode  string
	Summary        *MigrationSummary
	Config         ReportConfig
	Statistics     ReportStatistics
	RegistryStats  []RegistryStatistic
	ImagesByStatus []ImageStatus
	HasFailures    bool
	HasSkipped     bool
}

type ReportConfig struct {
	MultipleRegistries bool
	Concurrency        int
	Language           string
	TotalRegistries    int
	EnabledRegistries  []string
}

type ReportStatistics struct {
	TotalImages       int
	SuccessRate       float64
	FailureRate       float64
	SkippedRate       float64
	ProcessingTime    string
	AverageImageSize  string
	TopSourceRegistry string
	TopTargetRegistry string
}

type RegistryStatistic struct {
	Name         string
	Type         string
	Priority     int
	ImagesCount  int
	SuccessCount int
	FailureCount int
	SuccessRate  float64
}

type ImageStatus struct {
	SourceImage  string
	TargetImage  string
	Registry     string
	Status       string
	StatusClass  string
	Error        string
	ResourceType string
	Namespace    string
	Container    string
}
