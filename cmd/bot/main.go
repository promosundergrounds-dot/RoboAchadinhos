package main

import (
    "fmt"
    "log"
    "time"

    "github.com/spf13/viper"
    "underground/robo-achadinhos/internal/converter"
    "underground/robo-achadinhos/internal/database"
    "underground/robo-achadinhos/internal/scraper"
    "underground/robo-achadinhos/internal/sender"
)

func main() {
	// 1. Load configuration from .env
	viper.SetConfigFile(".env")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	botToken := viper.GetString("TELEGRAM_BOT_TOKEN")
	chatID := viper.GetString("TELEGRAM_CHAT_ID")
	dbPath := viper.GetString("DB_PATH")
	meliAffID := viper.GetString("MELI_AFFILIATE_ID") // Pegando seu ID do .env

	// 2. Initialize database
	db, err := database.NewDB(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// 3. Initialize components
	// Passamos o ID de afiliado para o Scraper (se você seguiu o passo anterior)
	mlScraper := &scraper.MercadoLivreScraper{AffiliateID: meliAffID}
	
	// Iniciamos o conversor separado para futuras lojas (Amazon, Magalu)
	meliConverter := converter.NewMeliConverter(meliAffID)
	
	tgSender := sender.NewTelegramSender(botToken, chatID)

	fmt.Println("🚀 Starting UNDERGROUND bot...")

	// Ticker for 10 minutes
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	// 4. Run cycle
	runCycle(mlScraper, meliConverter, db, tgSender)

	for range ticker.C {
		runCycle(mlScraper, meliConverter, db, tgSender)
	}
}

func runCycle(scr *scraper.MercadoLivreScraper, conv *converter.MeliConverter, db *database.DB, snd *sender.TelegramSender) {
	fmt.Println("🔍 Running scrape cycle...")

	products, err := scr.ScrapeOffers()
	if err != nil {
		log.Printf("❌ Error scraping: %v", err)
		return
	}

	for _, product := range products {
		// Aplica a conversão de afiliado antes de qualquer coisa
		product.AffiliateURL = conv.ConvertToAffiliate(product.OriginalURL)

		isNew, err := db.IsNew(product.ID)
		if err != nil {
			log.Printf("Error checking if new: %v", err)
			continue
		}

		if isNew {
			fmt.Printf("🎯 New deal found: %s\n", product.Title)
			
			err = snd.SendProduct(product)
			if err != nil {
				log.Printf("Error sending product: %v", err)
				continue
			}

			err = db.MarkAsPosted(product.ID)
			if err != nil {
				log.Printf("Error marking as posted: %v", err)
			}
		}
	}
}