package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	UserID       int    `json:"user_id"`
}

func main() {
	code := flag.String("code", "", "Authorization code from Mercado Libre callback")
	envPath := flag.String("env", ".env", "Path to .env file")
	flag.Parse()

	if *code == "" {
		fmt.Println("Usage: go run cmd/auth/main.go -code=YOUR_CODE")
		fmt.Println("\nExample:")
		fmt.Println("go run cmd/auth/main.go -code=TG-69f92cc86755940001221839-3205306103")
		os.Exit(1)
	}

	godotenv.Load(*envPath)

	clientID := os.Getenv("MELI_CLIENT_ID")
	clientSecret := os.Getenv("MELI_CLIENT_SECRET")
	redirectURI := os.Getenv("REDIRECT_URI")

	if clientID == "" || clientSecret == "" || redirectURI == "" {
		fmt.Println("Error: Missing required .env variables:")
		if clientID == "" {
			fmt.Println("  - MELI_CLIENT_ID")
		}
		if clientSecret == "" {
			fmt.Println("  - MELI_CLIENT_SECRET")
		}
		if redirectURI == "" {
			fmt.Println("  - REDIRECT_URI")
		}
		os.Exit(1)
	}

	fmt.Println("🔄 Exchanging authorization code for access token...")
	fmt.Printf("  Client ID: %s\n", clientID)
	fmt.Printf("  Redirect URI: %s\n", redirectURI)

	token, err := exchangeCode(clientID, clientSecret, redirectURI, *code)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Successfully obtained tokens!")
	fmt.Printf("  Access Token: %s\n", token.AccessToken[:20]+"...")
	fmt.Printf("  Refresh Token: %s\n", token.RefreshToken[:20]+"...")
	fmt.Printf("  Expires In: %d seconds\n", token.ExpiresIn)
	fmt.Printf("  User ID: %d\n", token.UserID)

	if err := updateEnvFile(*envPath, token); err != nil {
		fmt.Printf("❌ Error updating .env: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ .env file updated at %s\n", *envPath)
	fmt.Println("\nYou can now run the bot:")
	fmt.Println("  go run cmd/bot/main.go")
}

func exchangeCode(clientID, clientSecret, redirectURI, code string) (*TokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	resp, err := http.PostForm(
		"https://api.mercadolibre.com/oauth/token",
		form,
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oauth failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var token TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}

	return &token, nil
}

func updateEnvFile(path string, token *TokenResponse) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	updates := map[string]string{
		"MELI_ACCESS_TOKEN":  token.AccessToken,
		"MELI_REFRESH_TOKEN": token.RefreshToken,
	}

	for key, value := range updates {
		found := false
		for idx, line := range lines {
			if strings.HasPrefix(line, key+"=") {
				lines[idx] = fmt.Sprintf("%s=%s", key, value)
				found = true
				break
			}
		}
		if !found {
			lines = append(lines, fmt.Sprintf("%s=%s", key, value))
		}
	}

	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	return os.WriteFile(path, []byte(content), 0o600)
}
