package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"log/slog"
	"underground/robo-achadinhos/docs"
	"underground/robo-achadinhos/internal/api"
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

	interval := getSearchInterval()
	logger.Info("bot started", "interval", interval.String())

	apiPort := getAPIPort()
	docs.SwaggerInfo.Host = fmt.Sprintf("localhost:%s", apiPort)
	docs.SwaggerInfo.BasePath = "/"

	router := gin.Default()
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	handler := api.NewHandler(meliClient, storageClient, logger)
	apiGroup := router.Group("/v1")
	meliGroup := apiGroup.Group("/meli")
	meliGroup.GET("/search", handler.SearchMeli)
	meliGroup.GET("/items/:id", handler.GetItem)
	meliGroup.POST("/affiliate", handler.CreateAffiliate)
	apiGroup.GET("/offers", handler.ListOffers)

	go func() {
		if err := router.Run(":" + apiPort); err != nil && err != http.ErrServerClosed {
			logger.Error("api server failed", "err", err, "port", apiPort)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	var running atomic.Bool
	go runCycle(ctx, cfg, meliClient, storageClient, telegramSender, logger, &running)

	ticker := time.NewTicker(interval)
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

	// Try to search promotions via API if token is available, fall back to scraping
	offers, err := meliClient.SearchPromotions(cycleCtx)
	if err != nil {
		logger.Warn("API promotion search failed, falling back to scraper", "err", err)
		offers, err = meliClient.SearchOffers(cycleCtx)
		if err != nil {
			logger.Error("failed to fetch offers", "err", err)
			return
		}
	}

	sent := false
	for _, offer := range offers {
		if !offerQualifies(offer) {
			continue
		}

		// Use MeliID if available, otherwise use regular ID
		checkID := offer.MeliID
		if checkID == "" {
			checkID = offer.ID
		}

		isNew, err := storageClient.IsNewOffer(cycleCtx, checkID)
		if err != nil {
			logger.Error("checking offer existence", "err", err, "offer_id", checkID)
			continue
		}

		if !isNew {
			logger.Debug("offer already posted", "offer_id", checkID)
			continue
		}

		// Verify item details and get coupon info if available
		if offer.MeliID != "" {
			itemDetails, err := meliClient.GetItemDetails(cycleCtx, offer.MeliID)
			if err != nil {
				logger.Warn("failed to fetch item details", "err", err, "item_id", offer.MeliID)
			} else {
				// Extract sale_terms (coupons) if available
				if saleTerms, ok := itemDetails["sale_terms"].([]interface{}); ok && len(saleTerms) > 0 {
					for _, term := range saleTerms {
						if termMap, ok := term.(map[string]interface{}); ok {
							if id, ok := termMap["id"].(string); ok && strings.Contains(strings.ToLower(id), "coupon") {
								if val, ok := termMap["value_name"].(string); ok {
									offer.Coupon = val
									break
								}
							}
						}
					}
				}
			}
		}

		// Generate affiliate short URL
		var affiliateURL string
		if offer.MeliID != "" && cfg.MELIAffiliateID != "" {
			longURL := offer.Permalink
			if longURL == "" {
				longURL = "https://www.mercadolivre.com.br/" + offer.MeliID
			}

			shortURL, err := meliClient.CreateShortURL(cycleCtx, longURL)
			if err != nil {
				logger.Warn("failed to create short URL, using long URL", "err", err, "item_id", offer.MeliID)
				affiliateURL = longURL
			} else {
				affiliateURL = shortURL
			}
		} else {
			affiliateURL = offer.AffiliateURL(cfg.MELIAffiliateID)
		}

		if err := telegramSender.SendOffer(cycleCtx, offer, affiliateURL); err != nil {
			logger.Error("failed to send telegram message", "err", err, "offer_id", checkID)
			continue
		}

		if err := storageClient.MarkAsPosted(cycleCtx, offer); err != nil {
			logger.Error("failed to save posted offer", "err", err, "offer_id", checkID)
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

func getSearchInterval() time.Duration {
	value := strings.TrimSpace(os.Getenv("SEARCH_INTERVAL"))
	if value == "" {
		return 1 * time.Minute
	}

	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 1 * time.Minute
	}
	return duration
}

func getAPIPort() string {
	port := strings.TrimSpace(os.Getenv("API_PORT"))
	if port == "" {
		return "8081"
	}
	return port
}

func offerQualifies(offer models.Offer) bool {
	if offer.OriginalPrice <= 0 {
		return false
	}
	// Permite produtos a partir de 2 reais para ser bem abrangente
	if offer.Price <= 2 {
		return false
	}
	discount := ((offer.OriginalPrice - offer.Price) / offer.OriginalPrice) * 100
	return discount >= 5 // 5% de desconto já é uma oferta
}
