package reporter

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

type HTMLReporter struct {
	logger     *logger.Logger
	reportsDir string
}

func NewHTMLReporter(logger *logger.Logger) *HTMLReporter {
	home, _ := os.UserHomeDir()
	reportsDir := filepath.Join(home, ".privateer", "reports")

	os.MkdirAll(reportsDir, 0755)

	return &HTMLReporter{
		logger:     logger,
		reportsDir: reportsDir,
	}
}

func (r *HTMLReporter) GenerateReport(summary *types.MigrationSummary, config *types.Config, isDryRun bool) (string, error) {
	timestamp := time.Now()
	filename := fmt.Sprintf("privateer-report-%s.html", timestamp.Format("2006-01-02_15-04-05"))
	if isDryRun {
		filename = fmt.Sprintf("privateer-dryrun-%s.html", timestamp.Format("2006-01-02_15-04-05"))
	}

	reportPath := filepath.Join(r.reportsDir, filename)

	data := r.buildReportData(summary, config, isDryRun, timestamp)

	htmlContent, err := r.generateHTML(data)
	if err != nil {
		return "", fmt.Errorf("falha ao gerar HTML: %w", err)
	}

	if err := os.WriteFile(reportPath, []byte(htmlContent), 0644); err != nil {
		return "", fmt.Errorf("falha ao salvar relatório: %w", err)
	}

	r.logger.Info("html_report_generated").
		Str("file", reportPath).
		Str("mode", getExecutionMode(isDryRun)).
		Int("total_images", summary.TotalImages).
		Send()

	return reportPath, nil
}

func (r *HTMLReporter) buildReportData(summary *types.MigrationSummary, config *types.Config, isDryRun bool, timestamp time.Time) types.ReportData {
	enabledRegistries := []string{}
	for _, reg := range config.Registries {
		if reg.Enabled {
			enabledRegistries = append(enabledRegistries, fmt.Sprintf("%s (priority: %d)", reg.Name, reg.Priority))
		}
	}

	return types.ReportData{
		Title:         getReportTitle(isDryRun),
		Timestamp:     timestamp.Format("2006-01-02 15:04:05"),
		ExecutionMode: getExecutionMode(isDryRun),
		Summary:       summary,
		Config: types.ReportConfig{
			MultipleRegistries: config.Settings.MultipleRegistries,
			Concurrency:        config.Settings.Concurrency,
			Language:           config.Settings.Language,
			TotalRegistries:    len(config.Registries),
			EnabledRegistries:  enabledRegistries,
		},
		Statistics:     r.calculateStatistics(summary),
		RegistryStats:  r.calculateRegistryStats(summary, config),
		ImagesByStatus: r.buildImageStatusList(summary),
		HasFailures:    summary.FailureCount > 0,
		HasSkipped:     summary.SkippedCount > 0,
	}
}

func (r *HTMLReporter) calculateStatistics(summary *types.MigrationSummary) types.ReportStatistics {
	total := float64(summary.TotalImages)
	if total == 0 {
		total = 1
	}

	successRate := float64(summary.SuccessCount) / total * 100
	failureRate := float64(summary.FailureCount) / total * 100
	skippedRate := float64(summary.SkippedCount) / total * 100

	registryCount := make(map[string]int)
	for _, result := range summary.Results {
		if result.Success {
			registryCount[result.Registry]++
		}
	}

	topRegistry := ""
	maxCount := 0
	for registry, count := range registryCount {
		if count > maxCount {
			maxCount = count
			topRegistry = registry
		}
	}

	return types.ReportStatistics{
		TotalImages:       summary.TotalImages,
		SuccessRate:       successRate,
		FailureRate:       failureRate,
		SkippedRate:       skippedRate,
		ProcessingTime:    "N/A",
		AverageImageSize:  "N/A",
		TopSourceRegistry: "DockerHub",
		TopTargetRegistry: topRegistry,
	}
}

func (r *HTMLReporter) calculateRegistryStats(summary *types.MigrationSummary, config *types.Config) []types.RegistryStatistic {
	registryStats := make(map[string]*types.RegistryStatistic)

	for _, reg := range config.Registries {
		if reg.Enabled {
			registryStats[reg.Name] = &types.RegistryStatistic{
				Name:         reg.Name,
				Type:         reg.Type,
				Priority:     reg.Priority,
				ImagesCount:  0,
				SuccessCount: 0,
				FailureCount: 0,
			}
		}
	}

	for _, result := range summary.Results {
		if stat, exists := registryStats[result.Registry]; exists {
			stat.ImagesCount++
			if result.Success {
				stat.SuccessCount++
			} else {
				stat.FailureCount++
			}
		}
	}

	var stats []types.RegistryStatistic
	for _, stat := range registryStats {
		if stat.ImagesCount > 0 {
			stat.SuccessRate = float64(stat.SuccessCount) / float64(stat.ImagesCount) * 100
		}
		stats = append(stats, *stat)
	}

	return stats
}

func (r *HTMLReporter) buildImageStatusList(summary *types.MigrationSummary) []types.ImageStatus {
	var images []types.ImageStatus

	for _, result := range summary.Results {
		status := "Sucesso"
		statusClass := "success"
		errorMsg := ""

		if result.Skipped {
			status = "Ignorado"
			statusClass = "warning"
			errorMsg = result.Reason
		} else if !result.Success {
			status = "Falha"
			statusClass = "danger"
			if result.Error != nil {
				errorMsg = result.Error.Error()
			}
		}

		images = append(images, types.ImageStatus{
			SourceImage:  result.Image.Image,
			TargetImage:  result.TargetImage,
			Registry:     result.Registry,
			Status:       status,
			StatusClass:  statusClass,
			Error:        errorMsg,
			ResourceType: result.Image.ResourceType,
			Namespace:    result.Image.Namespace,
			Container:    result.Image.Container,
		})
	}

	return images
}

func (r *HTMLReporter) generateHTML(data types.ReportData) (string, error) {
	tmpl := `<!DOCTYPE html>
<html lang="pt-BR">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}} - {{.Timestamp}}</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background: #f5f7fa; color: #333; line-height: 1.6; }
        .container { max-width: 1200px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; border-radius: 10px; margin-bottom: 30px; box-shadow: 0 10px 30px rgba(0,0,0,0.1); }
        .header h1 { font-size: 2.5rem; margin-bottom: 10px; text-shadow: 2px 2px 4px rgba(0,0,0,0.3); }
        .header p { font-size: 1.1rem; opacity: 0.9; }
        .stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 20px; margin-bottom: 30px; }
        .stat-card { background: white; padding: 25px; border-radius: 10px; box-shadow: 0 5px 15px rgba(0,0,0,0.08); border-left: 5px solid #667eea; }
        .stat-card h3 { color: #667eea; font-size: 2rem; margin-bottom: 5px; }
        .stat-card p { color: #666; font-weight: 500; }
        .section { background: white; margin-bottom: 30px; border-radius: 10px; overflow: hidden; box-shadow: 0 5px 15px rgba(0,0,0,0.08); }
        .section-header { background: #667eea; color: white; padding: 20px; font-size: 1.3rem; font-weight: 600; }
        .section-content { padding: 25px; }
        .table { width: 100%; border-collapse: collapse; margin-top: 10px; }
        .table th, .table td { padding: 12px; text-align: left; border-bottom: 1px solid #eee; }
        .table th { background: #f8f9fa; font-weight: 600; color: #333; }
        .table tr:hover { background: #f8f9fa; }
        .badge { padding: 4px 12px; border-radius: 20px; font-size: 0.85rem; font-weight: 500; }
        .badge.success { background: #d4edda; color: #155724; }
        .badge.warning { background: #fff3cd; color: #856404; }
        .badge.danger { background: #f8d7da; color: #721c24; }
        .progress-bar { width: 100%; height: 8px; background: #eee; border-radius: 4px; overflow: hidden; }
        .progress-fill { height: 100%; transition: width 0.3s ease; }
        .progress-success { background: #28a745; }
        .progress-warning { background: #ffc107; }
        .progress-danger { background: #dc3545; }
        .config-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; }
        .config-item { padding: 15px; background: #f8f9fa; border-radius: 8px; border-left: 3px solid #667eea; }
        .config-item strong { color: #667eea; }
        .footer { text-align: center; padding: 30px; color: #666; border-top: 1px solid #eee; margin-top: 30px; }
        .logo { font-size: 1.5rem; margin-right: 10px; }
        .truncate { max-width: 300px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
        @media (max-width: 768px) { .stats-grid { grid-template-columns: 1fr; } .table { font-size: 0.9rem; } }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1><span class="logo">🏴‍☠️</span>{{.Title}}</h1>
            <p>Relatório gerado em {{.Timestamp}} | Modo: {{.ExecutionMode}}</p>
        </div>

        <div class="stats-grid">
            <div class="stat-card">
                <h3>{{.Summary.TotalImages}}</h3>
                <p>Total de Imagens</p>
            </div>
            <div class="stat-card">
                <h3>{{.Summary.SuccessCount}}</h3>
                <p>Migrações Bem-sucedidas</p>
            </div>
            <div class="stat-card">
                <h3>{{.Summary.FailureCount}}</h3>
                <p>Falhas</p>
            </div>
            <div class="stat-card">
                <h3>{{printf "%.1f%%" .Statistics.SuccessRate}}</h3>
                <p>Taxa de Sucesso</p>
            </div>
        </div>

        <div class="section">
            <div class="section-header">📊 Estatísticas Detalhadas</div>
            <div class="section-content">
                <div style="margin-bottom: 20px;">
                    <div style="display: flex; justify-content: space-between; margin-bottom: 5px;">
                        <span>Taxa de Sucesso</span>
                        <span>{{printf "%.1f%%" .Statistics.SuccessRate}}</span>
                    </div>
                    <div class="progress-bar">
                        <div class="progress-fill progress-success" style="width: {{.Statistics.SuccessRate}}%"></div>
                    </div>
                </div>
                
                {{if .HasFailures}}
                <div style="margin-bottom: 20px;">
                    <div style="display: flex; justify-content: space-between; margin-bottom: 5px;">
                        <span>Taxa de Falhas</span>
                        <span>{{printf "%.1f%%" .Statistics.FailureRate}}</span>
                    </div>
                    <div class="progress-bar">
                        <div class="progress-fill progress-danger" style="width: {{.Statistics.FailureRate}}%"></div>
                    </div>
                </div>
                {{end}}
                
                {{if .HasSkipped}}
                <div style="margin-bottom: 20px;">
                    <div style="display: flex; justify-content: space-between; margin-bottom: 5px;">
                        <span>Taxa de Imagens Ignoradas</span>
                        <span>{{printf "%.1f%%" .Statistics.SkippedRate}}</span>
                    </div>
                    <div class="progress-bar">
                        <div class="progress-fill progress-warning" style="width: {{.Statistics.SkippedRate}}%"></div>
                    </div>
                </div>
                {{end}}
            </div>
        </div>

        <div class="section">
            <div class="section-header">⚙️ Configuração da Execução</div>
            <div class="section-content">
                <div class="config-grid">
                    <div class="config-item">
                        <strong>Múltiplos Registries:</strong><br>
                        {{if .Config.MultipleRegistries}}✅ Habilitado{{else}}❌ Desabilitado{{end}}
                    </div>
                    <div class="config-item">
                        <strong>Concorrência:</strong><br>
                        {{.Config.Concurrency}} threads
                    </div>
                    <div class="config-item">
                        <strong>Idioma:</strong><br>
                        {{.Config.Language}}
                    </div>
                    <div class="config-item">
                        <strong>Registries:</strong><br>
                        {{.Config.TotalRegistries}} configurados
                    </div>
                </div>
                
                <h4 style="margin: 20px 0 10px 0;">Registries Habilitados:</h4>
                <ul style="margin-left: 20px;">
                    {{range .Config.EnabledRegistries}}
                    <li>{{.}}</li>
                    {{end}}
                </ul>
            </div>
        </div>

        {{if .RegistryStats}}
        <div class="section">
            <div class="section-header">🎯 Estatísticas por Registry</div>
            <div class="section-content">
                <table class="table">
                    <thead>
                        <tr>
                            <th>Registry</th>
                            <th>Tipo</th>
                            <th>Prioridade</th>
                            <th>Imagens</th>
                            <th>Sucessos</th>
                            <th>Falhas</th>
                            <th>Taxa de Sucesso</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .RegistryStats}}
                        <tr>
                            <td><strong>{{.Name}}</strong></td>
                            <td><span class="badge success">{{.Type}}</span></td>
                            <td>{{.Priority}}</td>
                            <td>{{.ImagesCount}}</td>
                            <td style="color: #28a745;">{{.SuccessCount}}</td>
                            <td style="color: #dc3545;">{{.FailureCount}}</td>
                            <td>{{printf "%.1f%%" .SuccessRate}}</td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>
        {{end}}

        <div class="section">
            <div class="section-header">📋 Detalhes das Migrações</div>
            <div class="section-content">
                <table class="table">
                    <thead>
                        <tr>
                            <th>Imagem Origem</th>
                            <th>Imagem Destino</th>
                            <th>Registry</th>
                            <th>Status</th>
                            <th>Namespace</th>
                            <th>Recurso</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .ImagesByStatus}}
                        <tr>
                            <td class="truncate" title="{{.SourceImage}}">{{.SourceImage}}</td>
                            <td class="truncate" title="{{.TargetImage}}">{{.TargetImage}}</td>
                            <td><strong>{{.Registry}}</strong></td>
                            <td><span class="badge {{.StatusClass}}">{{.Status}}</span></td>
                            <td>{{.Namespace}}</td>
                            <td>{{.ResourceType}}</td>
                        </tr>
                        {{if .Error}}
                        <tr style="background: #fff3cd;">
                            <td colspan="6" style="font-size: 0.9rem; color: #856404;">
                                <strong>Erro:</strong> {{.Error}}
                            </td>
                        </tr>
                        {{end}}
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>

        <div class="footer">
            <p>🏴‍☠️ <strong>Privateer Migration Engine</strong> | Relatório gerado automaticamente</p>
            <p style="font-size: 0.9rem; margin-top: 10px;">
                Este relatório contém informações detalhadas sobre a migração de imagens Docker.<br>
                Para mais informações, consulte a documentação do Privateer.
            </p>
        </div>
    </div>
</body>
</html>`

	t, err := template.New("report").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func getReportTitle(isDryRun bool) string {
	if isDryRun {
		return "Privateer - Relatório de Simulação"
	}
	return "Privateer - Relatório de Migração"
}

func getExecutionMode(isDryRun bool) string {
	if isDryRun {
		return "Simulação (Dry Run)"
	}
	return "Produção (Real)"
}
