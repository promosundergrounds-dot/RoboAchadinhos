package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	meliSearchURL = "https://api.mercadolibre.com/sites/MLB/search?highlights=promotions&status=active&limit=15"
	tgSendPhotoURLTemplate = "https://api.telegram.org/bot%s/sendPhoto"
)

func main() {
	loadEnv()

	telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	telegramChatID := os.Getenv("TELEGRAM_CHAT_ID")
	meliAccessToken := os.Getenv("MELI_ACCESS_TOKEN")

	if telegramToken == "" || telegramChatID == "" || meliAccessToken == "" {
		slog.Error("Variáveis de ambiente obrigatórias não encontradas", "TELEGRAM_BOT_TOKEN", telegramToken == "", "TELEGRAM_CHAT_ID", telegramChatID == "", "MELI_ACCESS_TOKEN", meliAccessToken == "")
		os.Exit(1)
	}

	slog.Info("Buscando ofertas no Mercado Livre")
	offer, err := fetchBestAffiliateDeal(meliAccessToken)
	if err != nil {
		slog.Error("Erro ao buscar ofertas", "error", err)
		os.Exit(1)
	}

	if offer == nil {
		slog.Info("Nenhuma oferta encontrada que atenda aos critérios de curadoria")
		return
	}

	slog.Info("Oferta selecionada", "id", offer.ID, "title", offer.Title, "price", offer.Price, "original_price", offer.OriginalPrice)

	if err := sendTelegramPhoto(telegramToken, telegramChatID, offer); err != nil {
		slog.Error("Erro ao enviar oferta para Telegram", "error", err)
		os.Exit(1)
	}

	slog.Info("Oferta enviada com sucesso ao Telegram", "chat_id", telegramChatID)
}

func loadEnv() {
	if err := godotenv.Load(); err != nil {
		slog.Warn("Não foi possível carregar o arquivo .env; usando variáveis de ambiente do sistema se existirem", "error", err)
	}
}

func fetchBestAffiliateDeal(accessToken string) (*meliProduct, error) {
	req, err := http.NewRequest(http.MethodGet, meliSearchURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		slog.Error("Erro de autenticação. O token MELI_ACCESS_TOKEN provavelmente expirou", "status", resp.StatusCode)
		return nil, fmt.Errorf("autenticacao mercadolivre falhou: status %d", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("resposta inesperada do Mercado Livre: status %d", resp.StatusCode)
	}

	var response meliSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	for _, item := range response.Results {
		if passesFilters(item) {
			item.Thumbnail = upgradeThumbnail(item.Thumbnail)
			return &item, nil
		}
	}

	return nil, nil
}

func passesFilters(item meliProduct) bool {
	if item.OriginalPrice <= 0 {
		return false
	}
	if item.Price >= item.OriginalPrice {
		return false
	}
	if item.Price < 30 {
		return false
	}

	discount := ((item.OriginalPrice - item.Price) / item.OriginalPrice) * 100
	if discount < 20 {
		return false
	}

	return true
}

func upgradeThumbnail(thumbnail string) string {
	if thumbnail == "" {
		return thumbnail
	}
	return strings.Replace(thumbnail, "-I.jpg", "-F.jpg", 1)
}

func sendTelegramPhoto(token, chatID string, offer *meliProduct) error {
	url := fmt.Sprintf(tgSendPhotoURLTemplate, token)

	caption := fmt.Sprintf("*%s*\nPreço promocional: *R$ %.2f*", escapeMarkdown(offer.Title), offer.Price)

	payload := map[string]any{
		"chat_id":    chatID,
		"photo":      offer.Thumbnail,
		"caption":    caption,
		"parse_mode": "Markdown",
		"reply_markup": map[string]any{
			"inline_keyboard": []any{
				[]map[string]any{{
					"text": "🛒 IR PARA A LOJA",
					"url":  offer.Permalink,
				}},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var responseError map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&responseError)
		return fmt.Errorf("erro ao enviar mensagem ao Telegram: status %d, resposta %v", resp.StatusCode, responseError)
	}

	return nil
}

func escapeMarkdown(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(`", "\\(`",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(text)
}

type meliSearchResponse struct {
	Results []meliProduct `json:"results"`
}

type meliProduct struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Price         float64 `json:"price"`
	OriginalPrice float64 `json:"original_price"`
	Permalink     string  `json:"permalink"`
	Thumbnail     string  `json:"thumbnail"`
}
