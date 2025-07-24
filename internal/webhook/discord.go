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

type DiscordMessage struct {
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Content   string         `json:"content,omitempty"`
	Embeds    []DiscordEmbed `json:"embeds,omitempty"`
}

type DiscordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
	Footer      *DiscordEmbedFooter `json:"footer,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
}

type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type DiscordEmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

func NewDiscordWebhook(config types.DiscordWebhookConfig, logger *logger.Logger) *DiscordWebhook {
	name := config.Name
	if name == "" {
		name = "Privateer 🏴‍☠️"
	}

	avatar := config.Avatar
	if avatar == "" {
		avatar = "https://raw.githubusercontent.com/kevinfinalboss/privateer/main/.github/images/privateer-logo.png"
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

	embed := DiscordEmbed{
		Title:       operation,
		Description: "Iniciando migração de imagens Docker",
		Color:       color,
		Fields: []DiscordEmbedField{
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
		Footer: &DiscordEmbedFooter{
			Text: "Privateer Migration Engine",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	message := DiscordMessage{
		Username:  d.name,
		AvatarURL: d.avatar,
		Embeds:    []DiscordEmbed{embed},
	}

	return d.send(ctx, message)
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

	fields := []DiscordEmbedField{
		{
			Name: "📊 Resultados",
			Value: fmt.Sprintf("**Total:** %d\n**✅ Sucessos:** %d\n**❌ Falhas:** %d\n**⏭️ Ignoradas:** %d",
				summary.TotalImages, summary.SuccessCount, summary.FailureCount, summary.SkippedCount),
			Inline: true,
		},
	}

	successExamples := d.getSuccessExamples(summary.Results, 3)
	if len(successExamples) > 0 {
		fields = append(fields, DiscordEmbedField{
			Name:   "🎯 Migrações Realizadas",
			Value:  "```\n" + successExamples + "\n```",
			Inline: false,
		})
	}

	failureExamples := d.getFailureExamples(summary.Results, 3)
	if len(failureExamples) > 0 {
		fields = append(fields, DiscordEmbedField{
			Name:   "❌ Falhas",
			Value:  "```\n" + failureExamples + "\n```",
			Inline: false,
		})
	}

	embed := DiscordEmbed{
		Title:       operation,
		Description: description,
		Color:       color,
		Fields:      fields,
		Footer: &DiscordEmbedFooter{
			Text: "Privateer Migration Engine",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	message := DiscordMessage{
		Username:  d.name,
		AvatarURL: d.avatar,
		Embeds:    []DiscordEmbed{embed},
	}

	return d.send(ctx, message)
}

func (d *DiscordWebhook) SendError(ctx context.Context, errorMsg string, operation string) error {
	embed := DiscordEmbed{
		Title:       "❌ ERRO NA MIGRAÇÃO",
		Description: fmt.Sprintf("Falha durante: %s", operation),
		Color:       0xff0000,
		Fields: []DiscordEmbedField{
			{
				Name:   "💥 Erro",
				Value:  "```\n" + errorMsg + "\n```",
				Inline: false,
			},
		},
		Footer: &DiscordEmbedFooter{
			Text: "Privateer Migration Engine",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	message := DiscordMessage{
		Username:  d.name,
		AvatarURL: d.avatar,
		Embeds:    []DiscordEmbed{embed},
	}

	return d.send(ctx, message)
}

func (d *DiscordWebhook) send(ctx context.Context, message DiscordMessage) error {
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
