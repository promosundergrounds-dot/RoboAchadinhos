package main

import (
	"context"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"log/slog"
	"underground/robo-achadinhos/internal/config"
	"underground/robo-achadinhos/internal/meli"
	"underground/robo-achadinhos/internal/models"
	"underground/robo-achadinhos/internal/storage"
	"underground/robo-achadinhos/internal/telegram"
)

func main() {
	logger := config.NewLogger()

	cfg, err := config.LoadConfig(".env")
	if err != nil {
		logger.Error("loading configuration", "err", err)
		os.Exit(1)
	}

	storageClient, err := storage.NewStorage(cfg.DBPath, logger)
	if err != nil {
		logger.Error("initializing storage", "err", err)
		os.Exit(1)
	}
	defer storageClient.Close()

	meliClient := meli.NewClient(cfg, logger)

	telegramSender, err := telegram.NewSender(cfg.TelegramBotToken, cfg.TelegramChatID, logger)
	if err != nil {
		logger.Error("initializing telegram sender", "err", err)
		os.Exit(1)
	}

	logger.Info("bot started", "interval", "5m")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	var running atomic.Bool
	go runCycle(ctx, cfg, meliClient, storageClient, telegramSender, logger, &running)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("shutdown requested")
			return
		case <-ticker.C:
			go runCycle(ctx, cfg, meliClient, storageClient, telegramSender, logger, &running)
		}
	}
}

func runCycle(ctx context.Context, cfg *config.Config, meliClient *meli.MeliClient, storageClient *storage.Storage, telegramSender *telegram.Sender, logger *slog.Logger, running *atomic.Bool) {
	if !running.CompareAndSwap(false, true) {
		logger.Warn("previous cycle still running")
		return
	}
	defer running.Store(false)

	cycleCtx, cancel := ensureMinTimeout(ctx, 2*time.Minute)
	defer cancel()

	logger.Info("starting search cycle")

	offers, err := meliClient.SearchOffers(cycleCtx)
	if err != nil {
		logger.Error("failed to fetch offers", "err", err)
		return
	}

	sent := false
	for _, offer := range offers {
		if !offerQualifies(offer) {
			continue
		}

		isNew, err := storageClient.IsNewOffer(cycleCtx, offer.ID)
		if err != nil {
			logger.Error("checking offer existence", "err", err, "offer_id", offer.ID)
			continue
		}

		if !isNew {
			continue
		}

		affiliateURL := offer.AffiliateURL(cfg.MELIAffiliateID)
		if err := telegramSender.SendOffer(cycleCtx, offer, affiliateURL); err != nil {
			logger.Error("failed to send telegram message", "err", err, "offer_id", offer.ID)
			continue
		}

		if err := storageClient.MarkAsPosted(cycleCtx, offer); err != nil {
			logger.Error("failed to save posted offer", "err", err, "offer_id", offer.ID)
		}

		sent = true
		break
	}

	if !sent {
		logger.Info("no qualifying offer found this cycle")
	}

	logger.Info("search cycle completed", "offers_total", len(offers))
}

func ensureMinTimeout(ctx context.Context, min time.Duration) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) >= min {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, min)
}

func offerQualifies(offer models.Offer) bool {
	if offer.OriginalPrice <= 0 {
		return false
	}

	if offer.Price < 30 {
		return false
	}

	discount := ((offer.OriginalPrice - offer.Price) / offer.OriginalPrice) * 100
	return discount >= 20
}
