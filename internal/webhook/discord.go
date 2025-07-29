package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

type DiscordWebhook struct {
	url    string
	name   string
	avatar string
	logger *logger.Logger
	client *http.Client
}

func NewDiscordWebhook(config types.DiscordWebhookConfig, logger *logger.Logger) *DiscordWebhook {
	name := config.Name
	if name == "" {
		name = "Privateer 🏴‍☠️"
	}

	avatar := config.Avatar
	if avatar == "" {
		avatar = "https://raw.githubusercontent.com/kevinfinalboss/privateer/refs/heads/master/.github/images/privateer-logo.png"
	}

	return &DiscordWebhook{
		url:    config.URL,
		name:   name,
		avatar: avatar,
		logger: logger,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (d *DiscordWebhook) SendMigrationStart(ctx context.Context, totalImages int, registries []string, dryRun bool) error {
	operation := "🚀 MIGRAÇÃO INICIADA"
	color := 0x00ff00

	if dryRun {
		operation = "🧪 SIMULAÇÃO INICIADA"
		color = 0xffaa00
	}

	embed := types.DiscordEmbed{
		Title:       operation,
		Description: "Iniciando migração de imagens Docker",
		Color:       color,
		Fields: []types.DiscordEmbedField{
			{
				Name:   "📦 Imagens",
				Value:  fmt.Sprintf("%d imagens públicas encontradas", totalImages),
				Inline: true,
			},
			{
				Name:   "🎯 Registries de Destino",
				Value:  "```\n" + strings.Join(registries, "\n") + "\n```",
				Inline: false,
			},
			{
				Name:   "⚙️ Modo",
				Value:  getModeText(dryRun),
				Inline: true,
			},
		},
		Footer: &types.DiscordEmbedFooter{
			Text: "Privateer Migration Engine",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	message := types.DiscordMessage{
		Username:  d.name,
		AvatarURL: d.avatar,
		Embeds:    []types.DiscordEmbed{embed},
	}

	return d.send(ctx, message)
}

func (d *DiscordWebhook) SendMessage(ctx context.Context, message types.DiscordMessage) error {
	return d.send(ctx, message)
}

func (d *DiscordWebhook) GetName() string {
	return d.name
}

func (d *DiscordWebhook) GetAvatar() string {
	return d.avatar
}

func (d *DiscordWebhook) SendMigrationComplete(ctx context.Context, summary *types.MigrationSummary, dryRun bool) error {
	operation := "✅ MIGRAÇÃO CONCLUÍDA"
	color := 0x00ff00

	if dryRun {
		operation = "✅ SIMULAÇÃO CONCLUÍDA"
		color = 0x0099ff
	}

	if summary.FailureCount > 0 {
		operation = "⚠️ MIGRAÇÃO COM FALHAS"
		color = 0xff6600
	}

	description := fmt.Sprintf("Processo finalizado com %d sucessos", summary.SuccessCount)
	if summary.FailureCount > 0 {
		description += fmt.Sprintf(" e %d falhas", summary.FailureCount)
	}
	if summary.SkippedCount > 0 {
		description += fmt.Sprintf(" (%d ignoradas)", summary.SkippedCount)
	}

	fields := []types.DiscordEmbedField{
		{
			Name: "📊 Resultados",
			Value: fmt.Sprintf("**Total:** %d\n**✅ Sucessos:** %d\n**❌ Falhas:** %d\n**⏭️ Ignoradas:** %d",
				summary.TotalImages, summary.SuccessCount, summary.FailureCount, summary.SkippedCount),
			Inline: true,
		},
	}

	successExamples := d.getSuccessExamples(summary.Results, 3)
	if len(successExamples) > 0 {
		fields = append(fields, types.DiscordEmbedField{
			Name:   "🎯 Migrações Realizadas",
			Value:  "```\n" + successExamples + "\n```",
			Inline: false,
		})
	}

	failureExamples := d.getFailureExamples(summary.Results, 3)
	if len(failureExamples) > 0 {
		fields = append(fields, types.DiscordEmbedField{
			Name:   "❌ Falhas",
			Value:  "```\n" + failureExamples + "\n```",
			Inline: false,
		})
	}

	embed := types.DiscordEmbed{
		Title:       operation,
		Description: description,
		Color:       color,
		Fields:      fields,
		Footer: &types.DiscordEmbedFooter{
			Text: "Privateer Migration Engine",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	message := types.DiscordMessage{
		Username:  d.name,
		AvatarURL: d.avatar,
		Embeds:    []types.DiscordEmbed{embed},
	}

	return d.send(ctx, message)
}

func (d *DiscordWebhook) SendError(ctx context.Context, errorMsg string, operation string) error {
	embed := types.DiscordEmbed{
		Title:       "❌ ERRO NA MIGRAÇÃO",
		Description: fmt.Sprintf("Falha durante: %s", operation),
		Color:       0xff0000,
		Fields: []types.DiscordEmbedField{
			{
				Name:   "💥 Erro",
				Value:  "```\n" + errorMsg + "\n```",
				Inline: false,
			},
		},
		Footer: &types.DiscordEmbedFooter{
			Text: "Privateer Migration Engine",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	message := types.DiscordMessage{
		Username:  d.name,
		AvatarURL: d.avatar,
		Embeds:    []types.DiscordEmbed{embed},
	}

	return d.send(ctx, message)
}

func (d *DiscordWebhook) send(ctx context.Context, message types.DiscordMessage) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("falha ao serializar mensagem Discord: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", d.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("falha ao criar requisição Discord: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("falha ao enviar webhook Discord: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Discord retornou status %d", resp.StatusCode)
	}

	d.logger.Debug("discord_webhook_sent").
		Int("status_code", resp.StatusCode).
		Send()

	return nil
}

func (d *DiscordWebhook) getSuccessExamples(results []*types.MigrationResult, limit int) string {
	var examples []string
	count := 0

	for _, result := range results {
		if result.Success && count < limit {
			examples = append(examples, fmt.Sprintf("%s → %s",
				truncateString(result.Image.Image, 30),
				truncateString(result.TargetImage, 30)))
			count++
		}
	}

	if len(examples) == 0 {
		return ""
	}

	result := strings.Join(examples, "\n")
	if count < len(results) {
		result += fmt.Sprintf("\n... e mais %d migrações", len(results)-count)
	}

	return result
}

func (d *DiscordWebhook) getFailureExamples(results []*types.MigrationResult, limit int) string {
	var examples []string
	count := 0

	for _, result := range results {
		if !result.Success && !result.Skipped && count < limit {
			errorMsg := "erro desconhecido"
			if result.Error != nil {
				errorMsg = result.Error.Error()
			}
			examples = append(examples, fmt.Sprintf("%s: %s",
				truncateString(result.Image.Image, 25),
				truncateString(errorMsg, 40)))
			count++
		}
	}

	if len(examples) == 0 {
		return ""
	}

	return strings.Join(examples, "\n")
}

func getModeText(dryRun bool) string {
	if dryRun {
		return "🧪 Simulação (Dry Run)"
	}
	return "🚀 Produção (Real)"
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
