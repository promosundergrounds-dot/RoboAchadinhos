# UNDERGROUND - Robô de Ofertas para Afiliados

Um robô de alta performance para coletar e enviar ofertas de produtos de afiliados via Telegram.

## Funcionalidades

- **Scraping**: Coleta ofertas do Mercado Livre (simulado por enquanto).
- **Persistência**: Usa SQLite para evitar duplicatas.
- **Envio**: Simula envio para Telegram (console por enquanto).
- **Configuração**: Carrega variáveis de ambiente via Viper.

## Estrutura do Projeto

- `cmd/bot/main.go`: Ponto de entrada, orquestrador.
- `internal/models/product.go`: Modelo de dados do produto.
- `internal/database/db.go`: Lógica de banco de dados.
- `internal/scraper/meli.go`: Scraper do Mercado Livre.
- `internal/sender/telegram.go`: Envio para Telegram.

## Como Usar

1. Configure o `.env` com suas credenciais:
   ```
   TELEGRAM_BOT_TOKEN=your_bot_token
   TELEGRAM_CHAT_ID=@your_channel
   DB_PATH=offers.db
   ```

2. Instale dependências:
   ```bash
   go mod tidy
   ```

3. Execute:
   ```bash
   go run ./cmd/bot
   ```

O bot roda um ciclo a cada 10 minutos, scrapea ofertas, filtra duplicatas e simula envio.

## Notas Técnicas

- Usa `modernc.org/sqlite` sem CGO.
- Separação de responsabilidades com injeção de dependência.
- Tratamento gracioso de erros de rede.
- Para scraping real do Mercado Livre, considere usar um browser headless, pois a página carrega dinamicamente.

## Próximos Passos

- Implementar scraping real com chromedp.
- Adicionar Amazon e Magalu.
- Integrar envio real para Telegram.
- Adicionar logs e métricas.