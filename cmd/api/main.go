package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/quotahub/ucpqa/internal/app"
	"github.com/quotahub/ucpqa/internal/config"
	"github.com/quotahub/ucpqa/internal/domain/provider"
	"github.com/quotahub/ucpqa/internal/infrastructure/metrics"
	"github.com/quotahub/ucpqa/internal/infrastructure/providers/codex"
	"github.com/quotahub/ucpqa/internal/infrastructure/providers/kimi"
	"github.com/quotahub/ucpqa/internal/infrastructure/providers/minimax"
	"github.com/quotahub/ucpqa/internal/infrastructure/providers/monitorquota"
	"github.com/quotahub/ucpqa/internal/infrastructure/store"
	"github.com/quotahub/ucpqa/internal/runtime/syncmanager"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load("", "config.yaml")
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validating config: %w", err)
	}

	st := store.NewWithConfig(cfg.Global.MaxStaleDuration)
	m := metrics.New()

	var providers []provider.Provider
	for alias, provCfg := range cfg.Providers {
		providerType := provCfg.Type
		if providerType == "" {
			providerType = alias
		}

		switch strings.ToLower(providerType) {
		case "codex":
			providers = append(providers, codex.New(alias, provCfg.BaseURL, provCfg.Token))
		case "kimi":
			providers = append(providers, kimi.New(alias, provCfg.BaseURL, provCfg.Token))
		case "minimax":
			providers = append(providers, minimax.New(alias, provCfg.BaseURL, provCfg.Token))
		case "zai":
			providers = append(providers, monitorquota.NewZAI(alias, provCfg.BaseURL, provCfg.Token))
		case "zhipu":
			providers = append(providers, monitorquota.NewZhipu(alias, provCfg.BaseURL, provCfg.Token))
		default:
			log.Printf("[main] warning: unknown provider type %q for alias %q, skipping", providerType, alias)
			continue
		}
	}

	if len(providers) == 0 {
		log.Printf("[main] warning: no providers configured")
	}

	sm := syncmanager.New(providers, st, cfg)

	builder := &app.Builder{
		Config:      cfg,
		Store:       st,
		Metrics:     m,
		SyncManager: sm,
	}

	composition, err := builder.Build()
	if err != nil {
		return fmt.Errorf("building app: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("[main] received signal %v, initiating shutdown", sig)
		cancel()
	}()

	log.Printf("[main] starting API server on port %d, metrics on port %d",
		cfg.Server.APIPort, cfg.Server.MetricsPort)

	return composition.Run(ctx)
}
