# 🤖 RoboAchadinhos - Bot de Afiliados do Mercado Livre

Um bot de **alta performance e padrão profissional** para coletar ofertas de afiliados do Mercado Livre e enviar automaticamente via Telegram.

## ✨ Funcionalidades Principais

- **🔍 API do Mercado Livre**: Integração completa com refresh automático de token (fallback para API pública)
- **💾 SQLite**: Deduplicação de ofertas usando banco de dados local
- **🔐 Autenticação OAuth 2.0**: Suporte completo com renovação automática de tokens
- **📱 Telegram**: Envio de ofertas formatadas em HTML para canais ou usuários
- **⏰ Scheduler**: Execução a cada 5 minutos com proteção contra execução simultânea
- **📊 Logging Estruturado**: Logs em JSON com slog (Go 1.21+)
- **🚀 Zero CGO**: Usa `modernc.org/sqlite` - compilável em qualquer plataforma

## 📁 Arquitetura do Projeto

```
robo-achadinhos/
├── cmd/
│   ├── bot/
│   │   └── main.go           # Orquestrador principal do bot
│   └── auth/
│       └── main.go           # Utilitário para autenticar com Mercado Livre
├── internal/
│   ├── config/
│   │   └── config.go         # Carregamento e validação de .env
│   ├── models/
│   │   └── product.go        # Estrutura de Offer e métodos
│   ├── meli/
│   │   └── client.go         # Cliente HTTP da API do Mercado Livre com retry automático
│   ├── storage/
│   │   └── sqlite.go         # Persistência e deduplicação
│   └── telegram/
│       └── telegram.go       # Integração com Telegram Bot API
├── go.mod / go.sum           # Dependências Go
├── .env                       # Variáveis de ambiente (não commitar)
└── README.md
```

## 🔧 Variáveis de Ambiente Obrigatórias

```bash
# Telegram
TELEGRAM_BOT_TOKEN=seu_token_aqui
TELEGRAM_CHAT_ID=@seu_canal_ou_id_numerico

# Banco de Dados
DB_PATH=offers.db

# Mercado Livre (Afiliado)
MELI_AFFILIATE_ID=seu_affiliate_id
REDIRECT_URI=https://seu-dominio/callback

# Mercado Livre (Autenticação)
MELI_ACCESS_TOKEN=seu_access_token
MELI_REFRESH_TOKEN=seu_refresh_token (opcional inicialmente)
MELI_CLIENT_ID=seu_client_id (opcional se usar tokens apenas)
MELI_CLIENT_SECRET=seu_client_secret (opcional se usar tokens apenas)
```

## 🚀 Como Usar

### 1️⃣ Clonar e Instalar Dependências

```bash
cd robo-achadinhos
go mod tidy
```

### 2️⃣ Configurar Autenticação com Mercado Livre

Se você não tiver um `MELI_ACCESS_TOKEN` válido:

1. Crie uma aplicação em https://applications.mercadolibre.com/
2. Obtenha `MELI_CLIENT_ID` e `MELI_CLIENT_SECRET`
3. Configure o `REDIRECT_URI` (pode ser uma página estática)
4. Acesse: 
   ```
   https://auth.mercadolibre.com.br/authorization?response_type=code&client_id=SEU_CLIENT_ID&redirect_uri=SEU_REDIRECT_URI
   ```
5. Você receberá um código de autorização na URL
6. Troque o código pelos tokens:
   ```bash
   go run cmd/auth/main.go -code=TG-seu_codigo_aqui
   ```

Isso atualiza automaticamente seu `.env` com os tokens válidos.

### 3️⃣ Executar o Bot

```bash
go run ./cmd/bot/main.go
```

O bot irá:
1. Carregar configuração do `.env`
2. Inicializar banco de dados SQLite
3. Conectar à API do Mercado Livre
4. Iniciar ciclo de 5 minutos:
   - ✅ Buscar ofertas da API
   - ✅ Filtrar duplicatas (banco de dados)
   - ✅ Converter para link de afiliado
   - ✅ Enviar para Telegram
   - ✅ Salvar no banco de dados

## 🏗️ Arquitetura Técnica

### `internal/config`
- Carrega `.env` com `godotenv`
- Valida variáveis obrigatórias
- Fornece método `SaveEnv()` para atualizar tokens
- Logger estruturado com `slog`

### `internal/meli`
- Cliente HTTP com retry automático
- **RoundTripper personalizado**: Se receber 401, tenta renovar token automaticamente
- Suporta fallback para API pública se token expirar
- Métodos:
  - `NewClient()`: Criar cliente
  - `SearchOffers()`: Buscar ofertas com fallback

### `internal/storage`
- SQLite com schema simples
- Métodos:
  - `IsNewOffer()`: Verifica se já foi enviado
  - `MarkAsPosted()`: Marca como enviado

### `internal/telegram`
- Telegram Bot API v5
- Suporta canais (`@UndergroundPromos`) e IDs numéricos
- Formatação HTML com links de afiliado

### `cmd/bot/main.go`
- Signal handling (Ctrl+C)
- Execução concorrente com Goroutines
- Proteção contra overlapping (atomic.Bool)
- Timeout de 2 minutos por ciclo

### `cmd/auth/main.go`
- Utilitário standalone para autenticação
- Troca código de autorização por tokens
- Atualiza `.env` automaticamente

## 📊 Fluxo de Funcionamento

```
┌─────────────────────────────────────────────────────────┐
│ 1. Bot Inicia                                           │
│    ↓ LoadConfig() → valida .env                        │
│    ↓ NewStorage() → SQLite                             │
│    ↓ NewClient() → Meli com RoundTripper               │
└─────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────┐
│ 2. Ticker a cada 5 minutos                              │
│    (Se não houver execução anterior)                    │
└─────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────┐
│ 3. SearchOffers()                                       │
│    ↓ Tenta com token autenticado                       │
│    ↓ Se 401 → RefreshToken() automático                │
│    ↓ Se ainda falhar → Fallback para API pública       │
└─────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────┐
│ 4. Para cada oferta:                                    │
│    ↓ IsNewOffer() → Verifica no SQLite                 │
│    ↓ Se novo → AffiliateURL() → Gera link com ID      │
│    ↓ SendOffer() → Envia para Telegram                 │
│    ↓ MarkAsPosted() → Salva no banco                   │
└─────────────────────────────────────────────────────────┘
```

## 🔐 Renovação Automática de Token

O bot implementa um **RoundTripper personalizado** que:

1. Intercepta todas as requisições HTTP
2. Se receber 401 (token expirado):
   - Executa refresh automático
   - Atualiza token em memória e `.env`
   - Refaz a requisição original
   - Transparente para o chamador

```go
if resp.StatusCode == http.StatusUnauthorized {
    refreshAccessToken()  // Automático
    retry()               // Tenta novamente
}
```

## 📦 Dependências

```bash
github.com/go-telegram-bot-api/telegram-bot-api/v5
github.com/joho/godotenv
modernc.org/sqlite (pure Go, sem CGO)
```

## 🛠️ Desenvolvimento

### Build para Produção

```bash
go build -o bot.exe ./cmd/bot
go build -o auth.exe ./cmd/auth
```

### Executar Testes

```bash
go test ./...
```

### Compilar para Diferentes Plataformas

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o bot ./cmd/bot

# macOS
GOOS=darwin GOARCH=arm64 go build -o bot ./cmd/bot

# Windows
GOOS=windows GOARCH=amd64 go build -o bot.exe ./cmd/bot
```

## 📈 Próximos Passos

- [ ] Adicionar suporte a Amazon e Magalu
- [ ] Implementar métricas de Prometheus
- [ ] Adicionar dashboard com ofertas em tempo real
- [ ] Suporte a múltiplos canais Telegram
- [ ] Cache de ofertas com TTL
- [ ] Filtros por categoria e preço
- [ ] Alertas de anomalia (preços muito altos/baixos)

## 📝 Licença

MIT

## 👤 Autor

UNDERGROUND - Bot de Ofertas de Afiliados