package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"underground/robo-achadinhos/internal/meli"

	"github.com/joho/godotenv"
)

func main() {
	envPath := flag.String("env", ".env", "Path to .env file")
	flag.Parse()

	if err := godotenv.Load(*envPath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Error loading .env: %v\n", err)
		os.Exit(1)
	}

	clientID := strings.TrimSpace(os.Getenv("MELI_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("MELI_CLIENT_SECRET"))
	redirectURI := firstNonEmpty(strings.TrimSpace(os.Getenv("MELI_REDIRECT_URI")), strings.TrimSpace(os.Getenv("REDIRECT_URI")))

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

	authorizeURL := buildAuthorizeURL(clientID, redirectURI)

	fmt.Println("✅ HTTP server will listen on http://localhost:8080")
	fmt.Println("✅ Abra esta URL no browser para autorizar:")
	fmt.Println(authorizeURL)
	fmt.Println("\nAguardando callback do Mercado Livre com ?code=...")

	codeCh := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			http.Error(w, "Missing code query parameter", http.StatusBadRequest)
			return
		}

		fmt.Printf("🔔 Código recebido: %s\n", code)
		fmt.Fprintln(w, "Código recebido. Pode fechar esta aba.")
		select {
		case codeCh <- code:
		default:
		}
	})

	srv := &http.Server{Addr: ":8080", Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server failed: %v\n", err)
			os.Exit(1)
		}
	}()

	code := <-codeCh

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Printf("Warning: server shutdown failed: %v\n", err)
	}

	fmt.Println("🔄 Exchanging authorization code for access token...")
	token, err := meli.ExchangeCode(*envPath, code)
	if err != nil {
		fmt.Printf("❌ Error exchanging code: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Successfully obtained tokens!")
	fmt.Printf("  Access Token: %s\n", token.AccessToken[:20]+"...")
	fmt.Printf("  Refresh Token: %s\n", token.RefreshToken[:20]+"...")
	fmt.Printf("  Expires In: %d seconds\n", token.ExpiresIn)
	fmt.Printf("  User ID: %d\n", token.UserID)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func buildAuthorizeURL(clientID, redirectURI string) string {
	endpoint := "https://auth.mercadolibre.com.br/authorization"
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", "offline_access read write")

	return endpoint + "?" + params.Encode()
}
