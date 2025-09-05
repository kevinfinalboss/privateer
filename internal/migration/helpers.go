package migration

import (
	"context"
	"fmt"

	"github.com/kevinfinalboss/privateer/pkg/types"
)

func (e *Engine) updateSummaryCounters(summary *types.MigrationSummary, result *types.MigrationResult) {
	if result.Success {
		summary.SuccessCount++
		e.logger.Debug("summary_counter_updated").
			Str("type", "success").
			Int("new_count", summary.SuccessCount).
			Send()
	} else if result.Skipped {
		summary.SkippedCount++
		e.logger.Debug("summary_counter_updated").
			Str("type", "skipped").
			Int("new_count", summary.SkippedCount).
			Str("reason", result.Reason).
			Send()
	} else {
		summary.FailureCount++
		e.logger.Debug("summary_counter_updated").
			Str("type", "failure").
			Int("new_count", summary.FailureCount).
			Send()
		if result.Error != nil {
			summary.Errors = append(summary.Errors, result.Error)
		}
	}
}

func (e *Engine) logInputAnalysis(images []*types.ImageInfo) {
	e.logger.Debug("migration_input_analysis").
		Int("total_input_images", len(images)).
		Send()

	for i, img := range images {
		e.logger.Debug("input_image_details").
			Int("index", i).
			Str("image", img.Image).
			Str("namespace", img.Namespace).
			Str("resource_name", img.ResourceName).
			Str("resource_type", img.ResourceType).
			Bool("is_public", img.IsPublic).
			Bool("is_init_container", img.IsInitContainer).
			Str("container", img.Container).
			Send()
	}
}

func (e *Engine) logMigrationStart(images []*types.ImageInfo, targetRegistries []types.RegistryConfig) {
	e.logger.Info("migration_started_preserve_namespace").
		Int("total_images", len(images)).
		Int("target_registries", len(targetRegistries)).
		Strs("registries", getRegistryNames(targetRegistries)).
		Bool("multiple_registries", e.config.Settings.MultipleRegistries).
		Int("concurrency", e.concurrency).
		Str("strategy", "preserve_full_namespace").
		Send()
}

func (e *Engine) logMigrationComplete(summary *types.MigrationSummary) {
	e.logger.Info("migration_completed").
		Int("total", summary.TotalImages).
		Int("success", summary.SuccessCount).
		Int("failures", summary.FailureCount).
		Int("skipped", summary.SkippedCount).
		Send()
}

func (e *Engine) handleNoRegistriesError(ctx context.Context) (*types.MigrationSummary, error) {
	err := fmt.Errorf("nenhum registry habilitado encontrado")
	e.logger.Error("no_enabled_registries").Send()

	if e.discordWebhook != nil {
		e.discordWebhook.SendError(ctx, err.Error(), "Seleção de Registries")
	}

	return nil, err
}

func (e *Engine) sendDiscordComplete(ctx context.Context, summary *types.MigrationSummary) {
	if e.discordWebhook != nil {
		err := e.discordWebhook.SendMigrationComplete(ctx, summary, false)
		if err != nil {
			e.logger.Warn("discord_webhook_failed").Err(err).Send()
		}
	}
}

func (e *Engine) generateReport(summary *types.MigrationSummary, isDryRun bool) {
	reportPath, err := e.htmlReporter.GenerateReport(summary, e.config, isDryRun)
	if err != nil {
		e.logger.Warn("html_report_failed").Err(err).Send()
	} else {
		message := "Relatório HTML de migração gerado"
		if isDryRun {
			message = "Relatório HTML de simulação gerado"
		}
		e.logger.Info("html_report_ready").
			Str("path", reportPath).
			Str("message", message).
			Send()
	}
}

func getRegistryNames(registries []types.RegistryConfig) []string {
	names := make([]string, len(registries))
	for i, reg := range registries {
		names[i] = fmt.Sprintf("%s (priority: %d)", reg.Name, reg.Priority)
	}
	return names
}

func maskWebhookURL(url string) string {
	if len(url) < 20 {
		return "***"
	}
	return url[:20] + "***"
}
