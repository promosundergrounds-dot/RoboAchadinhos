package scraper

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"underground/robo-achadinhos/internal/models"
)

func SearchOffers(ctx context.Context) ([]models.Offer, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(ctx,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-dev-shm-usage", true),
		)...,
	)
	defer cancelAllocator()

	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	defer cancelBrowser()

	var rawProducts []map[string]string
	pageURL := "https://www.mercadolivre.com.br/ofertas"

	script := `(function() {
		const products = [];
		document.querySelectorAll('.promotion-item').forEach(item => {
			const titleEl = item.querySelector('h2') || item.querySelector('.promotion-item__title') || item.querySelector('.promotion-item__name') || item.querySelector('a');
			const linkEl = item.querySelector('a');
			const imageEl = item.querySelector('img');
			const originalEl = item.querySelector('.promotion-item__price--before, .promotion-item__original-price, .promotion-item__price-before, .promotion-item__old-price, .promotion-item__price__before');
			const priceEl = item.querySelector('.promotion-item__price--current, .promotion-item__price--sale, .promotion-item__price, .promotion-item__final-price, .promotion-item__discounted-price, .promotion-item__price__current');
			const title = titleEl ? titleEl.innerText.trim() : '';
			const permalink = linkEl ? linkEl.href : '';
			let thumbnail = '';
			if (imageEl) {
				thumbnail = imageEl.src || imageEl.dataset.src || imageEl.getAttribute('src') || '';
			}
			const original_price = originalEl ? originalEl.innerText.trim() : '';
			const price = priceEl ? priceEl.innerText.trim() : '';
			products.push({
				title,
				original_price,
				price,
				permalink,
				thumbnail,
			});
		});
		return products;
	})()`

	err := chromedp.Run(browserCtx,
		chromedp.Navigate(pageURL),
		chromedp.WaitVisible(`.promotion-item`, chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(script, &rawProducts),
	)
	if err != nil {
		return nil, err
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
