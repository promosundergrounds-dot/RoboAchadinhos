package scraper

import (
	"context"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"underground/robo-achadinhos/internal/models"

	"github.com/gocolly/colly/v2"
)

func SearchOffers(ctx context.Context) ([]models.Offer, error) {
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) < 2*time.Minute {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
	}

	collector := colly.NewCollector(
		colly.AllowedDomains("www.mercadolivre.com.br", "mercadolivre.com.br"),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"),
	)
	collector.SetRequestTimeout(60 * time.Second)
	_ = collector.Limit(&colly.LimitRule{
		DomainGlob:  "*.mercadolivre.com.br",
		Parallelism: 1,
		Delay:       2 * time.Second,
	})

	var rawProducts []map[string]string
	var mu sync.Mutex
	var scrapeErr error

	collector.OnRequest(func(r *colly.Request) {
		if ctx.Err() != nil {
			r.Abort()
		}
	})

	collector.OnError(func(_ *colly.Response, err error) {
		if scrapeErr == nil {
			scrapeErr = err
		}
	})

	collector.OnHTML(".poly-card__content", func(e *colly.HTMLElement) {
		title := strings.TrimSpace(e.ChildText(".poly-component__title"))
		permalink := strings.TrimSpace(e.ChildAttr(".poly-component__title", "href"))
		thumbnail := strings.TrimSpace(e.ChildAttr(".poly-component__picture", "src"))
		if thumbnail == "" {
			thumbnail = strings.TrimSpace(e.ChildAttr(".poly-component__picture", "data-src"))
		}
		if thumbnail == "" {
			thumbnail = strings.TrimSpace(e.ChildAttr(".poly-component__picture", "data-id"))
		}
		if thumbnail == "" {
			thumbnail = strings.TrimSpace(e.ChildAttr("img.poly-component__picture", "src"))
		}
		if thumbnail == "" {
			thumbnail = strings.TrimSpace(e.ChildAttr("img.poly-component__picture", "data-src"))
		}
		seller := strings.TrimSpace(e.ChildText(".ui-search-official-store-label, .ui-search-official-store-tag, .promotion-item__seller, .promotion-item__seller-name, .ui-search-item__group__element, .andes-badge__text, .ui-search-badge__subtitle, .ui-search-badge__text"))
		fullBadge := strings.TrimSpace(e.ChildText(".ui-search-full, .promotion-item__fulfillment, .promotion-item__badge-full, .promotion-item__badge, .andes-badge__text, .ui-search-badge__subtitle, .ui-search-badge__text"))
		originalPriceRaw := strings.TrimSpace(e.ChildText(".andes-money-amount--previous .andes-money-amount__fraction"))
		priceRaw := strings.TrimSpace(e.ChildText(".poly-price__current .andes-money-amount__fraction"))

		mu.Lock()
		rawProducts = append(rawProducts, map[string]string{
			"title":          title,
			"permalink":      permalink,
			"thumbnail":      thumbnail,
			"seller":         seller,
			"full":           fullBadge,
			"original_price": originalPriceRaw,
			"price":          priceRaw,
		})
		mu.Unlock()
	})

	visitErr := make(chan error, 1)
	go func() {
		visitErr <- collector.Visit("https://www.mercadolivre.com.br/ofertas")
		collector.Wait()
	}()

	select {
	case err := <-visitErr:
		if err != nil {
			return nil, err
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if scrapeErr != nil {
		return nil, scrapeErr
	}

	minDiscount := getMinDiscountPercent()
	type scoredOffer struct {
		offer    models.Offer
		score    int
		priority string
	}

	scoredOffers := make([]scoredOffer, 0, len(rawProducts))
	for _, raw := range rawProducts {
		price := parseCurrency(raw["price"])
		originalPrice := parseCurrency(raw["original_price"])
		discount := 0.0
		if originalPrice > 0 {
			discount = ((originalPrice - price) / originalPrice) * 100
		}
		seller := strings.TrimSpace(raw["seller"])
		full := strings.Contains(strings.ToLower(raw["full"]), "full")
		preferredSeller := isPreferredSeller(seller)
		priority := "normal"
		score := 0
		if full {
			score += 100
			priority = "full"
		}
		if preferredSeller {
			score += 10
			if priority == "normal" {
				priority = "preferred_seller"
			}
		}

		log.Printf("[DEBUG] Analisando produto: %s | Preço: %.2f", raw["title"], price)
		log.Printf("DEBUG product found: title=%q price=%.2f original=%.2f discount=%.2f%% seller=%q full=%t priority=%s permalink=%q",
			raw["title"], price, originalPrice, discount, seller, full, priority, raw["permalink"])

		if raw["title"] == "" || raw["permalink"] == "" {
			log.Printf("DEBUG discard: missing title or permalink seller=%q full=%t", seller, full)
			continue
		}
		if price <= 0 || originalPrice <= 0 {
			log.Printf("DEBUG discard: invalid pricing price=%.2f original=%.2f seller=%q full=%t", price, originalPrice, seller, full)
			continue
		}
		if discount < minDiscount {
			log.Printf("DEBUG discard: discount below threshold (%.2f%% < %.2f%%) seller=%q full=%t", discount, minDiscount, seller, full)
			continue
		}

		scoredOffers = append(scoredOffers, scoredOffer{
			offer: models.Offer{
				ID:            raw["permalink"],
				Title:         raw["title"],
				Price:         price,
				OriginalPrice: originalPrice,
				IsFull:        full,
				ImageURL:      upgradeThumbnail(raw["thumbnail"]),
				Permalink:     raw["permalink"],
			},
			score:    score,
			priority: priority,
		})
	}

	sort.SliceStable(scoredOffers, func(i, j int) bool {
		return scoredOffers[i].score > scoredOffers[j].score
	})

	offers := make([]models.Offer, 0, len(scoredOffers))
	for _, scored := range scoredOffers {
		offers = append(offers, scored.offer)
	}

	return offers, nil
}

func getMinDiscountPercent() float64 {
	const defaultDiscount = 5.0
	value := strings.TrimSpace(os.Getenv("MIN_DISCOUNT_PERCENT"))
	if value == "" {
		return defaultDiscount
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return defaultDiscount
	}
	return parsed
}

func isPreferredSeller(seller string) bool {
	lower := strings.ToLower(strings.TrimSpace(seller))
	return strings.Contains(lower, "loja oficial") || strings.Contains(lower, "lojas oficiais") || strings.Contains(lower, "mercado líder platinum") || strings.Contains(lower, "mercado lider platinum") || strings.Contains(lower, "mercado líder") || strings.Contains(lower, "mercado lider")
}

func parseCurrency(value string) float64 {
	if value == "" {
		return 0
	}

	clean := strings.TrimSpace(value)
	clean = strings.ReplaceAll(clean, "R$", "")
	clean = strings.ReplaceAll(clean, "r$", "")
	clean = strings.ReplaceAll(clean, ".", "")
	clean = strings.ReplaceAll(clean, "\u00A0", "")
	clean = strings.ReplaceAll(clean, ",", ".")
	clean = strings.TrimSpace(clean)
	clean = strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '.' || r == '-' {
			return r
		}
		return -1
	}, clean)

	if clean == "" {
		return 0
	}

	parsed, err := strconv.ParseFloat(clean, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func upgradeThumbnail(thumbnail string) string {
	if thumbnail == "" {
		return thumbnail
	}
	return strings.Replace(thumbnail, "-I.jpg", "-F.jpg", 1)
}
