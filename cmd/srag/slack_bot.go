package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// SlackBot implements the Bot interface for Slack
type SlackBot struct {
	botToken   string
	appToken   string
	botManager *BotManager
	botName    string
	botUserID  string
	ctx        context.Context
	cancel     context.CancelFunc
}

// SlackEvent represents a Slack event
type SlackEvent struct {
	Type    string      `json:"type"`
	Event   interface{} `json:"event"`
	TeamID  string      `json:"team_id"`
	EventID string      `json:"event_id"`
}

// SlackMessage represents a Slack message event
type SlackMessage struct {
	Type      string `json:"type"`
	Channel   string `json:"channel"`
	User      string `json:"user"`
	Text      string `json:"text"`
	Timestamp string `json:"ts"`
	ThreadTS  string `json:"thread_ts,omitempty"`
}

// SlackPostMessageRequest represents a request to post a message
type SlackPostMessageRequest struct {
	Channel   string `json:"channel"`
	Text      string `json:"text"`
	ThreadTS  string `json:"thread_ts,omitempty"`
	AsUser    bool   `json:"as_user,omitempty"`
}

// SlackAuthTestResponse represents the response from auth.test API
type SlackAuthTestResponse struct {
	Ok     bool   `json:"ok"`
	UserID string `json:"user_id"`
	User   string `json:"user"`
	TeamID string `json:"team_id"`
	Team   string `json:"team"`
}

func NewSlackBot(botToken, appToken string, botManager *BotManager) *SlackBot {
	ctx, cancel := context.WithCancel(context.Background())
	return &SlackBot{
		botToken:   botToken,
		appToken:   appToken,
		botManager: botManager,
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (sb *SlackBot) Start(ctx context.Context) error {
	// Get bot info first
	err := sb.getBotInfo()
	if err != nil {
		return fmt.Errorf("failed to get bot info: %w", err)
	}
	
	log.Info().Str("bot_name", sb.botName).Str("bot_user_id", sb.botUserID).Msg("Slack bot started")
	
	// Start socket mode connection (simplified)
	go sb.connectSocketMode()
	
	// Wait for context cancellation
	<-ctx.Done()
	return nil
}

func (sb *SlackBot) Stop() error {
	sb.cancel()
	log.Info().Str("bot_name", sb.botName).Msg("Slack bot stopped")
	return nil
}

func (sb *SlackBot) SendMessage(userID, message string) error {
	return sb.postMessage(userID, message, "")
}

func (sb *SlackBot) GetBotName() string {
	return "slack:" + sb.botName
}

func (sb *SlackBot) getBotInfo() error {
	// This is a simplified implementation
	// In a real implementation, you would make an HTTP request to Slack API
	// For now, we'll use placeholder values
	sb.botName = "SlimRAGBot"
	sb.botUserID = "U1234567890"
	return nil
}

func (sb *SlackBot) connectSocketMode() {
	for {
		select {
		case <-sb.ctx.Done():
			return
		default:
			// This is a simplified implementation
			// In a real implementation, you would:
			// 1. Connect to Slack Socket Mode WebSocket
			// 2. Handle incoming events
			// 3. Acknowledge events
			// 4. Reconnect on disconnection
			
			// For demonstration purposes, we'll just sleep
			time.Sleep(1 * time.Second)
		}
	}
}

func (sb *SlackBot) processEvent(event SlackEvent) {
	if event.Type != "event_callback" {
		return
	}
	
	// In a real implementation, you would properly parse the event
	// For now, we'll use a simplified approach
	
	// This is a placeholder for event processing
	// In reality, you would parse the event.Event interface{} into a SlackMessage
}

func (sb *SlackBot) processMessage(message SlackMessage) {
	if message.Type != "message" || message.Text == "" {
		return
	}
	
	// Skip messages from the bot itself
	if message.User == sb.botUserID {
		return
	}
	
	// Check if the message mentions the bot
	query, isMention := sb.extractSlackMention(message.Text)
	if !isMention {
		// In channels, only respond to mentions
		// In DMs, respond to all messages
		if !sb.isDirectMessage(message.Channel) {
			return
		}
		query = message.Text
	}
	
	if strings.TrimSpace(query) == "" {
		return
	}
	
	// Generate unique request ID
	requestID := fmt.Sprintf("slack_%s_%s", message.Channel, message.Timestamp)
	userID := message.Channel
	
	// Enqueue the request
	sb.botManager.EnqueueRequest(requestID, query, userID, "slack", func(response string, err error) {
		if err != nil {
			log.Error().Err(err).Str("request_id", requestID).Msg("Error processing request")
			response = "Sorry, I encountered an error while processing your request."
		}
		
		// Send response back to user
		threadTS := message.ThreadTS
		if threadTS == "" {
			threadTS = message.Timestamp // Start a new thread
		}
		
		if sendErr := sb.postMessage(message.Channel, response, threadTS); sendErr != nil {
			log.Error().Err(sendErr).Str("request_id", requestID).Msg("Error sending response")
		}
	}, func(position int) {
		// Notify user about their position in queue
		queueMsg := fmt.Sprintf("Your request has been queued. You are #%d in line.", position)
		threadTS := message.ThreadTS
		if threadTS == "" {
			threadTS = message.Timestamp // Start a new thread
		}
		if sendErr := sb.postMessage(message.Channel, queueMsg, threadTS); sendErr != nil {
			log.Error().Err(sendErr).Str("request_id", requestID).Msg("Error sending queue notification")
		}
	})
	
	log.Info().Str("request_id", requestID).Str("query", query).Msg("Slack request queued")
}

func (sb *SlackBot) extractSlackMention(text string) (string, bool) {
	// Slack mentions are in the format <@U1234567890>
	botMention := fmt.Sprintf("<@%s>", sb.botUserID)
	
	if strings.Contains(text, botMention) {
		// Remove the mention and clean up
		query := strings.ReplaceAll(text, botMention, "")
		query = strings.TrimSpace(query)
		return query, true
	}
	
	// Also check for @botname mentions
	return extractMention(text, sb.botName)
}

func (sb *SlackBot) isDirectMessage(channel string) bool {
	// Direct message channels in Slack start with 'D'
	return strings.HasPrefix(channel, "D")
}

func (sb *SlackBot) postMessage(channel, text, threadTS string) error {
	// This is a simplified implementation
	// In a real implementation, you would make an HTTP POST request to:
	// https://slack.com/api/chat.postMessage
	// with the appropriate JSON payload and Authorization header
	
	log.Info().Str("channel", channel).Str("text", text).Str("thread_ts", threadTS).Msg("Sending Slack message")
	return nil
}