package models

// Product represents a product offer
type Product struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Price         float64 `json:"price"`
	ImageURL      string  `json:"image_url"`
	OriginalURL   string  `json:"original_url"`
	AffiliateURL  string  `json:"affiliate_url"`
	Store         string  `json:"store"`
}