package telegram

import (
	"context"
	"fmt"
	"html"
	"os"
	"strings"

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
	captionLines := []string{
		fmt.Sprintf("<b>🏷️ %s</b>", html.EscapeString(offer.Title)),
		fmt.Sprintf("💰 <b>Preço:</b> R$ %.2f", offer.Price),
	}

	if offer.OriginalPrice > offer.Price {
		captionLines = append(captionLines, fmt.Sprintf("📉 <b>%.0f%% OFF</b> — de <s>R$ %.2f</s>",
			((offer.OriginalPrice-offer.Price)/offer.OriginalPrice)*100,
			offer.OriginalPrice,
		))
	} else {
		captionLines = append(captionLines, fmt.Sprintf("📉 <b>Desconto disponível</b>"))
	}

	if offer.Coupon != "" {
		// Exibe o cupom real capturado com formatação de destaque
		captionLines = append(captionLines, fmt.Sprintf("🎟️ <b>CUPOM:</b> <code>%s</code>", html.EscapeString(offer.Coupon)))
	} else {
		// Fallback caso não tenha cupom específico capturado
		captionLines = append(captionLines, "")
	}

	if offer.IsFull {
		captionLines = append(captionLines, "🚚 <b>Frete Grátis</b>")
	}

	captionLines = append(captionLines,
		fmt.Sprintf("🔗 <a href=\"%s\">Link de Compra</a>", html.EscapeString(affiliateURL)),
	)

	caption := strings.Join(captionLines, "\n")

	imageURL := offer.ImageURL
	if imageURL == "" {
		// Try default image from env first, otherwise fall back to text message
		imageURL = strings.TrimSpace(os.Getenv("TELEGRAM_DEFAULT_IMAGE"))
		if imageURL == "" {
			text := fmt.Sprintf("<b>🏷️ %s</b>\n💰 <b>Preço:</b> R$ %.2f\n🎟️ Verifique se há cupom disponível na página!\n🔗 <a href=\"%s\">Link de Compra</a>",
				html.EscapeString(offer.Title),
				offer.Price,
				html.EscapeString(affiliateURL),
			)

			var msg tgbotapi.MessageConfig
			if strings.HasPrefix(s.chatID, "@") {
				msg = tgbotapi.NewMessageToChannel(strings.TrimPrefix(s.chatID, "@"), text)
			} else {
				msg = tgbotapi.NewMessage(parseChatID(s.chatID), text)
			}
			msg.ParseMode = "HTML"

			if _, err := s.bot.Send(msg); err != nil {
				return err
			}

			s.logger.Info("telegram fallback message sent", "offer_id", offer.ID, "chat_id", s.chatID)
			return nil
		}
	}

	var photoMsg tgbotapi.PhotoConfig
	if strings.HasPrefix(s.chatID, "@") {
		photoMsg = tgbotapi.NewPhotoToChannel(strings.TrimPrefix(s.chatID, "@"), tgbotapi.FileURL(imageURL))
	} else {
		photoMsg = tgbotapi.NewPhoto(parseChatID(s.chatID), tgbotapi.FileURL(imageURL))
	}

	photoMsg.Caption = caption
	photoMsg.ParseMode = "HTML"

	if _, err := s.bot.Send(photoMsg); err != nil {
		return err
	}

	s.logger.Info("telegram photo sent", "offer_id", offer.ID, "chat_id", s.chatID)
	return nil
}

func parseChatID(id string) int64 {
	var chatID int64
	fmt.Sscanf(id, "%d", &chatID)
	return chatID
}
