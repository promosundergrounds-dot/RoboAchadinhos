package models

import "net/url"

type Offer struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Price         float64 `json:"price"`
	OriginalPrice float64 `json:"original_price"`
	IsFull        bool    `json:"is_full"`
	ImageURL      string  `json:"image_url"`
	Permalink     string  `json:"permalink"`
}

func (o Offer) AffiliateURL(affiliateID string) string {
	parsed, err := url.Parse(o.Permalink)
	if err != nil {
		return o.Permalink
	}

	query := parsed.Query()
	query.Set("mkt_affiliate", affiliateID)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
