package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// TelegramBot implements the Bot interface for Telegram
type TelegramBot struct {
	token      string
	botManager *BotManager
	botName    string
	ctx        context.Context
	cancel     context.CancelFunc
}

// TelegramUpdate represents a Telegram update
type TelegramUpdate struct {
	UpdateID int              `json:"update_id"`
	Message  *TelegramMessage `json:"message"`
}

// TelegramMessage represents a Telegram message
type TelegramMessage struct {
	MessageID int           `json:"message_id"`
	From      *TelegramUser `json:"from"`
	Chat      *TelegramChat `json:"chat"`
	Date      int64         `json:"date"`
	Text      string        `json:"text"`
}

// TelegramUser represents a Telegram user
type TelegramUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// TelegramChat represents a Telegram chat
type TelegramChat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title,omitempty"`
}

// TelegramSendMessageRequest represents a request to send a message
type TelegramSendMessageRequest struct {
	ChatID    int64  `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// TelegramGetMeResponse represents the response from getMe API
type TelegramGetMeResponse struct {
	Ok     bool          `json:"ok"`
	Result *TelegramUser `json:"result"`
}

func NewTelegramBot(token string, botManager *BotManager) *TelegramBot {
	ctx, cancel := context.WithCancel(context.Background())
	return &TelegramBot{
		token:      token,
		botManager: botManager,
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (tb *TelegramBot) Start(ctx context.Context) error {
	// Get bot info first
	err := tb.getBotInfo()
	if err != nil {
		return fmt.Errorf("failed to get bot info: %w", err)
	}

	log.Info().Str("bot_name", tb.botName).Msg("Telegram bot started")

	// Start polling for updates
	go tb.pollUpdates()

	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

func (tb *TelegramBot) Stop() error {
	tb.cancel()
	log.Info().Str("bot_name", tb.botName).Msg("Telegram bot stopped")
	return nil
}

func (tb *TelegramBot) SendMessage(userID, message string) error {
	chatID, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	return tb.sendMessage(chatID, message)
}

func (tb *TelegramBot) GetBotName() string {
	return "telegram:" + tb.botName
}

func (tb *TelegramBot) getBotInfo() error {
	// This is a simplified implementation
	// In a real implementation, you would make an HTTP request to Telegram API
	// For now, we'll use a placeholder name
	tb.botName = "SlimRAGBot"
	return nil
}

func (tb *TelegramBot) pollUpdates() {
	offset := 0

	for {
		select {
		case <-tb.ctx.Done():
			return
		default:
			// This is a simplified implementation
			// In a real implementation, you would make HTTP requests to Telegram API
			// to get updates using long polling

			// For demonstration purposes, we'll just sleep
			time.Sleep(1 * time.Second)

			// In a real implementation, you would:
			// 1. Make GET request to https://api.telegram.org/bot{token}/getUpdates
			// 2. Parse the response
			// 3. Process each update
			// 4. Update the offset

			offset++ // Placeholder to avoid unused variable warning
		}
	}
}

func (tb *TelegramBot) processUpdate(update TelegramUpdate) {
	if update.Message == nil || update.Message.Text == "" {
		return
	}

	message := update.Message

	// Check if the message mentions the bot
	query, isMention := extractMention(message.Text, tb.botName)
	if !isMention {
		// In group chats, only respond to mentions
		if message.Chat.Type != "private" {
			return
		}
		// In private chats, respond to all messages
		query = message.Text
	}

	if strings.TrimSpace(query) == "" {
		return
	}

	// Generate unique request ID
	requestID := fmt.Sprintf("tg_%d_%d", message.Chat.ID, message.MessageID)
	userID := strconv.FormatInt(message.Chat.ID, 10)

	// Enqueue the request
	tb.botManager.EnqueueRequest(requestID, query, userID, "telegram", func(response string, err error) {
		if err != nil {
			log.Error().Err(err).Str("request_id", requestID).Msg("Error processing request")
			response = "Sorry, I encountered an error while processing your request."
		}

		// Send response back to user
		if sendErr := tb.sendMessage(message.Chat.ID, response); sendErr != nil {
			log.Error().Err(sendErr).Str("request_id", requestID).Msg("Error sending response")
		}
	}, func(position int) {
		// Notify user about their position in queue
		queueMsg := fmt.Sprintf("Your request has been queued. You are #%d in line.", position)
		if sendErr := tb.sendMessage(message.Chat.ID, queueMsg); sendErr != nil {
			log.Error().Err(sendErr).Str("request_id", requestID).Msg("Error sending queue notification")
		}
	})

	log.Info().Str("request_id", requestID).Str("query", query).Msg("Telegram request queued")
}

func (tb *TelegramBot) sendMessage(chatID int64, text string) error {
	// This is a simplified implementation
	// In a real implementation, you would make an HTTP POST request to:
	// https://api.telegram.org/bot{token}/sendMessage
	// with the appropriate JSON payload

	log.Info().Int64("chat_id", chatID).Str("text", text).Msg("Sending Telegram message")
	return nil
}
