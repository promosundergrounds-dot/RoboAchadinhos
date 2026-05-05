package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramBotToken string
	TelegramChatID   string // Can be numeric ID or channel name like @UndergroundPromos
	DBPath           string
	MELIAffiliateID  string
	RedirectURI      string
	MELIAccessToken  string
	MELIClientID     string
	MELIClientSecret string
	MELIRefreshToken string
	EnvPath          string
}

func LoadConfig(envPath string) (*Config, error) {
	if envPath == "" {
		envPath = ".env"
	}

	if err := godotenv.Load(envPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("loading env file: %w", err)
	}

	chatIDStr := strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID"))
	if chatIDStr == "" {
		return nil, fmt.Errorf("TELEGRAM_CHAT_ID is required")
	}

	cfg := &Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   chatIDStr,
		DBPath:           firstNonEmpty(os.Getenv("DB_PATH"), "offers.db"),
		MELIAffiliateID:  os.Getenv("MELI_AFFILIATE_ID"),
		RedirectURI:      os.Getenv("REDIRECT_URI"),
		MELIAccessToken:  os.Getenv("MELI_ACCESS_TOKEN"),
		MELIClientID:     os.Getenv("MELI_CLIENT_ID"),
		MELIClientSecret: os.Getenv("MELI_CLIENT_SECRET"),
		MELIRefreshToken: os.Getenv("MELI_REFRESH_TOKEN"),
		EnvPath:          envPath,
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (c *Config) validate() error {
	if c.TelegramBotToken == "" {
		return errors.New("TELEGRAM_BOT_TOKEN is required")
	}
	if c.TelegramChatID == "" {
		return errors.New("TELEGRAM_CHAT_ID is required")
	}
	if c.MELIAffiliateID == "" {
		return errors.New("MELI_AFFILIATE_ID is required")
	}
	if c.MELIAccessToken == "" {
		return errors.New("MELI_ACCESS_TOKEN is required")
	}
	if c.RedirectURI == "" {
		return errors.New("REDIRECT_URI is required")
	}
	return nil
}

func (c *Config) SaveEnv(key, value string) error {
	if err := os.Setenv(key, value); err != nil {
		return err
	}
	return writeEnv(c.EnvPath, key, value)
}

func writeEnv(path, key, value string) error {
	var lines []string
	if data, err := os.ReadFile(path); err == nil {
		lines = strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	updated := false
	for idx, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			lines[idx] = fmt.Sprintf("%s=%s", key, value)
			updated = true
		}
	}

	if !updated {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	return os.WriteFile(path, []byte(content), 0o600)
}

func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, nil))
}
