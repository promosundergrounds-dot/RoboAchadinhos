package meli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"log/slog"
	"underground/robo-achadinhos/internal/config"
	"underground/robo-achadinhos/internal/models"
)

type MeliClient struct {
	httpClient  *http.Client
	baseURL     string
	cfg         *config.Config
	logger      *slog.Logger
	accessToken string
	mu          sync.RWMutex
}

type authTransport struct {
	base   http.RoundTripper
	client *MeliClient
}

type searchResponse struct {
	Results []struct {
		ID        string  `json:"id"`
		Title     string  `json:"title"`
		Price     float64 `json:"price"`
		Thumbnail string  `json:"thumbnail"`
		Permalink string  `json:"permalink"`
	} `json:"results"`
}

type refreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func NewClient(cfg *config.Config, logger *slog.Logger) *MeliClient {
	client := &MeliClient{
		baseURL:     "https://api.mercadolibre.com",
		cfg:         cfg,
		logger:      logger,
		accessToken: cfg.MELIAccessToken,
	}

	client.httpClient = &http.Client{
		Timeout:   25 * time.Second,
		Transport: &authTransport{base: http.DefaultTransport, client: client},
	}

	return client
}

func (c *MeliClient) SearchOffers(ctx context.Context) ([]models.Offer, error) {
	// Try with authentication first
	offers, err := c.searchWithAuth(ctx)
	if err == nil {
		return offers, nil
	}

	c.logger.Warn("authenticated search failed, trying without authentication", "error", err)
	return c.searchWithoutAuth(ctx)
}

func (c *MeliClient) searchWithAuth(ctx context.Context) ([]models.Offer, error) {
	endpoint, err := url.Parse(c.baseURL + "/sites/MLB/search")
	if err != nil {
		return nil, err
	}

	query := endpoint.Query()
	query.Set("q", "")
	query.Set("limit", "50")
	query.Set("offset", "0")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.getAccessToken())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("authenticated search failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return c.parseSearchResponse(resp)
}

func (c *MeliClient) searchWithoutAuth(ctx context.Context) ([]models.Offer, error) {
	endpoint, err := url.Parse(c.baseURL + "/sites/MLB/search")
	if err != nil {
		return nil, err
	}

	query := endpoint.Query()
	query.Set("q", "")
	query.Set("limit", "50")
	query.Set("offset", "0")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("public search failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return c.parseSearchResponse(resp)
}

func (c *MeliClient) parseSearchResponse(resp *http.Response) ([]models.Offer, error) {
	var parsed searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	offers := make([]models.Offer, 0, len(parsed.Results))
	for _, item := range parsed.Results {
		offers = append(offers, models.Offer{
			ID:        item.ID,
			Title:     item.Title,
			Price:     item.Price,
			ImageURL:  item.Thumbnail,
			Permalink: item.Permalink,
		})
	}

	c.logger.Info("offers fetched", "count", len(offers))
	return offers, nil
}

func (c *MeliClient) refreshAccessToken(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cfg.MELIClientID == "" || c.cfg.MELIClientSecret == "" || c.cfg.MELIRefreshToken == "" {
		return fmt.Errorf("refresh token credentials not configured; token may be expired")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", c.cfg.MELIClientID)
	form.Set("client_secret", c.cfg.MELIClientSecret)
	form.Set("refresh_token", c.cfg.MELIRefreshToken)
	form.Set("redirect_uri", c.cfg.RedirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rawClient := &http.Client{Timeout: 25 * time.Second}
	resp, err := rawClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("refresh token failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload refreshTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}

	if payload.AccessToken == "" {
		return fmt.Errorf("refresh token returned empty access token")
	}

	c.accessToken = payload.AccessToken
	c.cfg.MELIAccessToken = payload.AccessToken

	if err := c.cfg.SaveEnv("MELI_ACCESS_TOKEN", payload.AccessToken); err != nil {
		c.logger.Warn("failed to persist access token", "error", err)
	}

	if payload.RefreshToken != "" {
		c.cfg.MELIRefreshToken = payload.RefreshToken
		if err := c.cfg.SaveEnv("MELI_REFRESH_TOKEN", payload.RefreshToken); err != nil {
			c.logger.Warn("failed to persist refresh token", "error", err)
		}
	}

	return nil
}

func (c *MeliClient) getAccessToken() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.accessToken
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned, err := cloneRequest(req)
	if err != nil {
		return nil, err
	}

	if cloned.Header.Get("Authorization") == "" {
		cloned.Header.Set("Authorization", "Bearer "+t.client.getAccessToken())
	}

	resp, err := t.base.RoundTrip(cloned)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	_ = resp.Body.Close()
	if err := t.client.refreshAccessToken(req.Context()); err != nil {
		t.client.logger.Warn("failed to refresh access token", "error", err)
		body, _ := io.ReadAll(resp.Body)
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Status:     http.StatusText(http.StatusUnauthorized),
			Header:     resp.Header,
			Body:       io.NopCloser(bytes.NewReader(body)),
		}, nil
	}

	retry, err := cloneRequest(req)
	if err != nil {
		return nil, err
	}
	retry.Header.Set("Authorization", "Bearer "+t.client.getAccessToken())
	return t.base.RoundTrip(retry)
}

func cloneRequest(req *http.Request) (*http.Request, error) {
	clone := req.Clone(req.Context())
	if req.Body == nil {
		return clone, nil
	}

	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		clone.Body = body
		return clone, nil
	}

	content, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	clone.Body = io.NopCloser(bytes.NewReader(content))
	req.Body = io.NopCloser(bytes.NewReader(content))

	return clone, nil
}
