package telegram

import (
	"context"
	"fmt"
	"html"

	"log/slog"
	"underground/robo-achadinhos/internal/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Sender struct {
	bot    *tgbotapi.BotAPI
	chatID string // Can be numeric ID or channel name
	logger *slog.Logger
}

func NewSender(token string, chatID string, logger *slog.Logger) (*Sender, error) {
	if token == "" {
		return nil, fmt.Errorf("telegram bot token is required")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &Sender{bot: bot, chatID: chatID, logger: logger}, nil
}

func (s *Sender) SendOffer(ctx context.Context, offer models.Offer, affiliateURL string) error {
	message := fmt.Sprintf(
		"<b>%s</b>\nPreço: R$ %.2f\n<a href=\"%s\">Ver oferta</a>\n\nID: %s",
		html.EscapeString(offer.Title),
		offer.Price,
		html.EscapeString(affiliateURL),
		html.EscapeString(offer.ID),
	)

	var msg tgbotapi.MessageConfig
	if s.chatID[0:1] == "@" {
		msg = tgbotapi.NewMessageToChannel(s.chatID, message)
	} else {
		msg = tgbotapi.NewMessage(parseChatID(s.chatID), message)
	}
	msg.ParseMode = "HTML"
	msg.DisableWebPagePreview = false

	if _, err := s.bot.Send(msg); err != nil {
		return err
	}

	s.logger.Info("telegram message sent", "offer_id", offer.ID, "chat_id", s.chatID)
	return nil
}

func parseChatID(id string) int64 {
	var chatID int64
	fmt.Sscanf(id, "%d", &chatID)
	return chatID
}
