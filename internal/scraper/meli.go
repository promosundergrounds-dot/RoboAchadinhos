package scraper

import (
	"context"
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

	collector.OnHTML(".promotion-item", func(e *colly.HTMLElement) {
		title := strings.TrimSpace(e.ChildText("h2, .promotion-item__title, .promotion-item__name, a"))
		permalink := strings.TrimSpace(e.ChildAttr("a", "href"))
		thumbnail := strings.TrimSpace(e.ChildAttr("img", "src"))
		if thumbnail == "" {
			thumbnail = strings.TrimSpace(e.ChildAttr("img", "data-src"))
		}
		originalPriceRaw := strings.TrimSpace(e.ChildText(".promotion-item__price--before, .promotion-item__original-price, .promotion-item__price-before, .promotion-item__old-price, .promotion-item__price__before"))
		priceRaw := strings.TrimSpace(e.ChildText(".promotion-item__price--current, .promotion-item__price--sale, .promotion-item__price, .promotion-item__final-price, .promotion-item__discounted-price, .promotion-item__price__current"))

		mu.Lock()
		rawProducts = append(rawProducts, map[string]string{
			"title":          title,
			"permalink":      permalink,
			"thumbnail":      thumbnail,
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

	offers := make([]models.Offer, 0, len(rawProducts))
	for _, raw := range rawProducts {
		price := parseCurrency(raw["price"])
		originalPrice := parseCurrency(raw["original_price"])
		if raw["title"] == "" || raw["permalink"] == "" || price <= 0 {
			continue
		}

		offers = append(offers, models.Offer{
			ID:            raw["permalink"],
			Title:         raw["title"],
			Price:         price,
			OriginalPrice: originalPrice,
			ImageURL:      upgradeThumbnail(raw["thumbnail"]),
			Permalink:     raw["permalink"],
		})
	}

	return offers, nil
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
