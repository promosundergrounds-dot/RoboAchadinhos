package api

import (
	"net/http"
	"strconv"
	"strings"

	"log/slog"
	"underground/robo-achadinhos/internal/meli"
	"underground/robo-achadinhos/internal/models"
	"underground/robo-achadinhos/internal/storage"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	meli    *meli.MeliClient
	storage *storage.Storage
	logger  *slog.Logger
}

func NewHandler(meliClient *meli.MeliClient, storageClient *storage.Storage, logger *slog.Logger) *Handler {
	return &Handler{meli: meliClient, storage: storageClient, logger: logger}
}

type SearchOffersResponse struct {
	Offers []models.Offer `json:"offers"`
}

type ItemDetailResponse struct {
	Item              map[string]interface{} `json:"item"`
	Coupon            string                 `json:"coupon,omitempty"`
	AvailableQuantity int                    `json:"available_quantity,omitempty"`
	SoldQuantity      int                    `json:"sold_quantity,omitempty"`
}

type AffiliateRequest struct {
	URL string `json:"url" binding:"required,url"`
}

type AffiliateResponse struct {
	AffiliateURL string `json:"affiliate_url"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// SearchMeli godoc
// @Summary Search promotions in Mercado Livre
// @Description Search for promoted products using Mercado Livre API with optional filters.
// @Tags Meli
// @Param q query string true "Search query"
// @Param category query string false "Category ID"
// @Param min_price query int false "Minimum price"
// @Param max_price query int false "Maximum price"
// @Success 200 {object} SearchOffersResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/meli/search [get]
func (h *Handler) SearchMeli(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "q query parameter is required"})
		return
	}

	category := strings.TrimSpace(c.Query("category"))
	minPrice := parseIntQuery(c, "min_price")
	maxPrice := parseIntQuery(c, "max_price")

	offers, err := h.meli.SearchWithFilters(c.Request.Context(), query, category, minPrice, maxPrice)
	if err != nil {
		h.logger.Warn("search request failed", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, SearchOffersResponse{Offers: offers})
}

// GetItem godoc
// @Summary Get item details from Mercado Livre
// @Description Retrieve item details including coupon and stock information.
// @Tags Meli
// @Param id path string true "Item ID"
// @Success 200 {object} ItemDetailResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/meli/items/{id} [get]
func (h *Handler) GetItem(c *gin.Context) {
	itemID := strings.TrimSpace(c.Param("id"))
	if itemID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "item id is required"})
		return
	}

	itemDetails, err := h.meli.GetItemDetails(c.Request.Context(), itemID)
	if err != nil {
		h.logger.Warn("get item details failed", "error", err, "item_id", itemID)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	coupon := extractCoupon(itemDetails)
	availableQuantity := parseIntFromMap(itemDetails, "available_quantity")
	soldQuantity := parseIntFromMap(itemDetails, "sold_quantity")

	c.JSON(http.StatusOK, ItemDetailResponse{
		Item:              itemDetails,
		Coupon:            coupon,
		AvailableQuantity: availableQuantity,
		SoldQuantity:      soldQuantity,
	})
}

// CreateAffiliate godoc
// @Summary Create affiliate short URL
// @Description Generate an official afiliado short URL for a normal Mercado Livre URL.
// @Tags Meli
// @Accept json
// @Produce json
// @Param body body AffiliateRequest true "URL request"
// @Success 200 {object} AffiliateResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /v1/meli/affiliate [post]
func (h *Handler) CreateAffiliate(c *gin.Context) {
	var req AffiliateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	shortURL, err := h.meli.CreateShortURL(c.Request.Context(), req.URL)
	if err != nil {
		h.logger.Warn("create affiliate short url failed", "error", err, "url", req.URL)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, AffiliateResponse{AffiliateURL: shortURL})
}

// ListOffers godoc
// @Summary List saved offers
// @Description Retrieve offers already saved in SQLite.
// @Tags Offers
// @Success 200 {array} models.Offer
// @Failure 500 {object} ErrorResponse
// @Router /v1/offers [get]
func (h *Handler) ListOffers(c *gin.Context) {
	offers, err := h.storage.ListOffers(c.Request.Context())
	if err != nil {
		h.logger.Warn("list offers failed", "error", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, offers)
}

func parseIntQuery(c *gin.Context, key string) int {
	if value := strings.TrimSpace(c.Query(key)); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return 0
}

func parseIntFromMap(data map[string]interface{}, key string) int {
	if value, ok := data[key]; ok {
		switch typed := value.(type) {
		case float64:
			return int(typed)
		case int:
			return typed
		case int64:
			return int(typed)
		case string:
			if parsed, err := strconv.Atoi(typed); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func extractCoupon(itemDetails map[string]interface{}) string {
	if saleTerms, ok := itemDetails["sale_terms"].([]interface{}); ok {
		for _, term := range saleTerms {
			if termMap, ok := term.(map[string]interface{}); ok {
				if id, ok := termMap["id"].(string); ok && strings.Contains(strings.ToLower(id), "coupon") {
					if value, ok := termMap["value_name"].(string); ok {
						return value
					}
				}
			}
		}
	}
	return ""
}
