package scraper


import (
    "fmt"
    "strconv"
    "strings"

    "github.com/gocolly/colly/v2"
    "underground/robo-achadinhos/internal/models" // AJUSTE AQUI
)

type MercadoLivreScraper struct {
	AffiliateID string
}

func (s *MercadoLivreScraper) ScrapeOffers() ([]models.Product, error) {
	var products []models.Product
	c := colly.NewCollector(
		colly.AllowedDomains("www.mercadolivre.com.br"),
		colly.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"),
	)

	// Seletor para os cards de oferta do Mercado Livre
	// Seletor para os cards de oferta do Mercado Livre
// Seletor para os cards de oferta do Mercado Livre (conforme seu HTML)
c.OnHTML(".poly-card", func(e *colly.HTMLElement) {
    // 1. Link e Título
    titleElement := e.DOM.Find(".poly-component__title")
    link, _ := titleElement.Attr("href")
    title := titleElement.Text()
    
    // Limpa o link
    cleanLink := strings.Split(link, "#")[0]

    // 2. Preço (Ajustado para o novo padrão do ML)
    // O ML agora separa Real (.andes-money-amount__fraction) e Centavos (.andes-money-amount__cents)
    priceFraction := e.ChildText(".poly-price__current .andes-money-amount__fraction")
    priceCents := e.ChildText(".poly-price__current .andes-money-amount__cents")
    
    // Limpa pontos de milhar
    priceFraction = strings.ReplaceAll(priceFraction, ".", "")
    
    fullPriceStr := priceFraction
    if priceCents != "" {
        fullPriceStr = priceFraction + "." + priceCents
    }
    
    price, _ := strconv.ParseFloat(fullPriceStr, 64)

    // 3. Imagem
    imageURL := e.ChildAttr(".poly-component__picture", "src")
    if imageURL == "" {
        // Fallback para data-src caso tenha lazy load
        imageURL = e.ChildAttr(".poly-component__picture", "data-src")
    }

    product := models.Product{
        ID:           cleanLink,
        Title:        title,
        Price:        price,
        ImageURL:     imageURL,
        OriginalURL:  cleanLink,
        AffiliateURL: s.generateAffiliateLink(cleanLink),
        Store:        "Mercado Livre",
    }

    if product.Title != "" && product.Price > 0 {
        products = append(products, product)
    }
})

	err := c.Visit("https://www.mercadolivre.com.br/ofertas")
	if err != nil {
		return nil, err
	}

	return products, nil
}

// Lógica de Afiliação: Adiciona seu ID de parceiro
func (s *MercadoLivreScraper) generateAffiliateLink(originalURL string) string {
	// Se for um link oficial, o Meli aceita o parâmetro de afiliado
	separator := "?"
	if strings.Contains(originalURL, "?") {
		separator = "&"
	}
	return fmt.Sprintf("%s%spaid_revenue_id=%s", originalURL, separator, s.AffiliateID)
}