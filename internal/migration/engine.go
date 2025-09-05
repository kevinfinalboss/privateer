package migration

import (
	"context"
	"sync"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/kevinfinalboss/privateer/internal/registry"
	"github.com/kevinfinalboss/privateer/internal/reporter"
	"github.com/kevinfinalboss/privateer/internal/webhook"
	"github.com/kevinfinalboss/privateer/pkg/types"
)

type Engine struct {
	registryManager *registry.Manager
	logger          *logger.Logger
	config          *types.Config
	concurrency     int
	discordWebhook  *webhook.DiscordWebhook
	htmlReporter    *reporter.HTMLReporter
}

func NewEngine(registryManager *registry.Manager, logger *logger.Logger, cfg *types.Config) *Engine {
	concurrency := 3
	if cfg.Settings.Concurrency > 0 {
		concurrency = cfg.Settings.Concurrency
	}

	engine := &Engine{
		registryManager: registryManager,
		logger:          logger,
		config:          cfg,
		concurrency:     concurrency,
		htmlReporter:    reporter.NewHTMLReporter(logger),
	}

	if cfg.Webhooks.Discord.Enabled && cfg.Webhooks.Discord.URL != "" {
		engine.discordWebhook = webhook.NewDiscordWebhook(cfg.Webhooks.Discord, logger)
		engine.logger.Info("discord_webhook_enabled").
			Str("url", maskWebhookURL(cfg.Webhooks.Discord.URL)).
			Send()
	}

	return engine
}

func (e *Engine) MigrateImages(ctx context.Context, images []*types.ImageInfo) (*types.MigrationSummary, error) {
	if len(images) == 0 {
		e.logger.Info("no_images_to_migrate").Send()
		return &types.MigrationSummary{}, nil
	}

	e.logInputAnalysis(images)

	targetRegistries := e.selectTargetRegistries()
	if len(targetRegistries) == 0 {
		return e.handleNoRegistriesError(ctx)
	}

	e.logMigrationStart(images, targetRegistries)

	if e.discordWebhook != nil {
		err := e.discordWebhook.SendMigrationStart(ctx, len(images), getRegistryNames(targetRegistries), e.config.Settings.DryRun)
		if err != nil {
			e.logger.Warn("discord_webhook_failed").Err(err).Send()
		}
	}

	if e.config.Settings.DryRun {
		return e.executeDryRun(ctx, images, targetRegistries)
	}

	return e.executeRealMigration(ctx, images, targetRegistries)
}

func (e *Engine) executeRealMigration(ctx context.Context, images []*types.ImageInfo, targetRegistries []types.RegistryConfig) (*types.MigrationSummary, error) {
	summary := &types.MigrationSummary{
		TotalImages: len(images),
		Results:     make([]*types.MigrationResult, 0),
	}

	semaphore := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, image := range images {
		e.logger.Debug("processing_image_for_migration").
			Str("image", image.Image).
			Str("namespace", image.Namespace).
			Bool("multiple_registries", e.config.Settings.MultipleRegistries).
			Send()

		if e.config.Settings.MultipleRegistries {
			e.processImageForMultipleRegistries(ctx, image, targetRegistries, semaphore, &wg, &mu, summary)
		} else {
			e.processImageForSingleRegistry(ctx, image, targetRegistries[0], semaphore, &wg, &mu, summary)
		}
	}

	wg.Wait()

	e.logMigrationComplete(summary)
	e.sendDiscordComplete(ctx, summary)
	e.generateReport(summary, false)

	return summary, nil
}

func (e *Engine) executeDryRun(ctx context.Context, images []*types.ImageInfo, targetRegistries []types.RegistryConfig) (*types.MigrationSummary, error) {
	summary := e.dryRunMigration(images, targetRegistries)

	if e.discordWebhook != nil {
		err := e.discordWebhook.SendMigrationComplete(ctx, summary, true)
		if err != nil {
			e.logger.Warn("discord_webhook_failed").Err(err).Send()
		}
	}

	reportPath, err := e.htmlReporter.GenerateReport(summary, e.config, true)
	if err != nil {
		e.logger.Warn("html_report_failed").Err(err).Send()
	} else {
		e.logger.Info("html_report_ready").
			Str("path", reportPath).
			Str("message", "Relatório HTML de simulação gerado").
			Send()
	}

	return summary, nil
}

func (e *Engine) processImageForMultipleRegistries(ctx context.Context, image *types.ImageInfo, targetRegistries []types.RegistryConfig, semaphore chan struct{}, wg *sync.WaitGroup, mu *sync.Mutex, summary *types.MigrationSummary) {
	for _, regConfig := range targetRegistries {
		wg.Add(1)
		go func(img *types.ImageInfo, regCfg types.RegistryConfig) {
			defer wg.Done()

			e.logger.Debug("starting_goroutine_for_multiple_registries").
				Str("image", img.Image).
				Str("registry", regCfg.Name).
				Send()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := e.migrateImageToRegistry(ctx, img, regCfg.Name)

			mu.Lock()
			summary.Results = append(summary.Results, result)
			e.updateSummaryCounters(summary, result)
			mu.Unlock()
		}(image, regConfig)
	}
}

func (e *Engine) processImageForSingleRegistry(ctx context.Context, image *types.ImageInfo, registry types.RegistryConfig, semaphore chan struct{}, wg *sync.WaitGroup, mu *sync.Mutex, summary *types.MigrationSummary) {
	wg.Add(1)
	go func(img *types.ImageInfo) {
		defer wg.Done()

		e.logger.Debug("starting_goroutine_for_single_registry").
			Str("image", img.Image).
			Str("registry", registry.Name).
			Send()

		semaphore <- struct{}{}
		defer func() { <-semaphore }()

		result := e.migrateImageToRegistry(ctx, img, registry.Name)

		mu.Lock()
		summary.Results = append(summary.Results, result)
		e.updateSummaryCounters(summary, result)
		mu.Unlock()
	}(image)
}
