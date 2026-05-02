package sender

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"underground/robo-achadinhos/internal/models"
)

// TelegramSender handles sending messages to Telegram
type TelegramSender struct {
	Bot    *tgbotapi.BotAPI
	ChatID string
}

// NewTelegramSender creates a new TelegramSender and initializes the Bot
func NewTelegramSender(botToken, chatID string) *TelegramSender {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Falha ao conectar com o Telegram: %v", err)
	}

	return &TelegramSender{
		Bot:    bot,
		ChatID: chatID,
	}
}

// SendProduct envia a oferta real para o canal do Telegram
func (ts *TelegramSender) SendProduct(product models.Product) error {
	// Formatação Markdown
	message := fmt.Sprintf("*🔥 %s*\n\n💰 Preço: R$ %.2f\n\n🛒 [COMPRAR AGORA](%s)\n\n📌 Loja: %s",
		product.Title, product.Price, product.AffiliateURL, product.Store)

	// Cria a mensagem para o canal
	msg := tgbotapi.NewMessageToChannel(ts.ChatID, message)
	msg.ParseMode = tgbotapi.ModeMarkdown

	// Envio real
	_, err := ts.Bot.Send(msg)
	if err != nil {
		return fmt.Errorf("erro no envio via Telegram API: %w", err)
	}

	fmt.Printf("✅ Sucesso! Produto %s enviado para o canal.\n", product.ID)
	return nil
}