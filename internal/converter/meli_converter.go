package converter

import (
	"fmt"
	"strings"
)

// MeliConverter gerencia a transformação de links para afiliação
type MeliConverter struct {
	AffiliateID string
}

// NewMeliConverter inicializa o conversor com seu ID de parceiro
func NewMeliConverter(affiliateID string) *MeliConverter {
	return &MeliConverter{
		AffiliateID: affiliateID,
	}
}

// ConvertToAffiliate limpa a URL e adiciona o parâmetro de tracking do Meli
func (c *MeliConverter) ConvertToAffiliate(originalURL string) string {
	if c.AffiliateID == "" {
		return originalURL
	}

	// Remove parâmetros de busca originais e âncoras para evitar conflitos
	cleanURL := strings.Split(originalURL, "?")[0]
	cleanURL = strings.Split(cleanURL, "#")[0]

	// O Mercado Livre usa o paid_revenue_id para rastrear vendas de afiliados
	// Em uma fase futura, aqui você chamaria a API oficial para gerar links curtos
	return fmt.Sprintf("%s?paid_revenue_id=%s", cleanURL, c.AffiliateID)
}