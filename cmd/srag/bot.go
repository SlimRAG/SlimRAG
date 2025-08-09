package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"

	"github.com/fanyang89/rag/v1"
)

// RequestQueue implements a rate limiting and queuing mechanism for LLM requests
type RequestQueue struct {
	mu         sync.Mutex
	queue      []QueueItem
	maxWorkers int
	activeJobs int
	cond       *sync.Cond
	closed     bool
}

type QueueItem struct {
	ID        string
	Query     string
	UserID    string
	Platform  string
	Callback  func(response string, err error)
	CreatedAt time.Time
}

func NewRequestQueue(maxWorkers int) *RequestQueue {
	rq := &RequestQueue{
		maxWorkers: maxWorkers,
		queue:      make([]QueueItem, 0),
	}
	rq.cond = sync.NewCond(&rq.mu)
	return rq
}

func (rq *RequestQueue) Enqueue(item QueueItem) int {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	rq.queue = append(rq.queue, item)
	position := len(rq.queue)
	rq.cond.Signal()
	log.Info().Str("id", item.ID).Str("platform", item.Platform).Int("position", position).Msg("Request queued")
	return position
}

func (rq *RequestQueue) Dequeue() *QueueItem {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	for (len(rq.queue) == 0 || rq.activeJobs >= rq.maxWorkers) && !rq.closed {
		rq.cond.Wait()
	}

	// Return nil if closed and no items in queue
	if rq.closed && len(rq.queue) == 0 {
		return nil
	}

	if len(rq.queue) > 0 {
		item := rq.queue[0]
		rq.queue = rq.queue[1:]
		rq.activeJobs++
		return &item
	}
	return nil
}

func (rq *RequestQueue) MarkComplete() {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	rq.activeJobs--
	rq.cond.Signal()
}

func (rq *RequestQueue) GetQueueStatus() (queueLength int, activeJobs int) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	return len(rq.queue), rq.activeJobs
}

func (rq *RequestQueue) Close() {
	rq.mu.Lock()
	defer rq.mu.Unlock()

	rq.closed = true
	rq.cond.Broadcast() // Wake up all waiting goroutines
}

// Bot represents a generic chat bot interface
type Bot interface {
	Start(ctx context.Context) error
	Stop() error
	SendMessage(userID, message string) error
	GetBotName() string
}

// BotManager manages multiple bot instances and request processing
type BotManager struct {
	bots         []Bot
	rag          *rag.RAG
	requestQueue *RequestQueue
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewBotManager(r *rag.RAG, maxWorkers int) *BotManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &BotManager{
		bots:         make([]Bot, 0),
		rag:          r,
		requestQueue: NewRequestQueue(maxWorkers),
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (bm *BotManager) AddBot(bot Bot) {
	bm.bots = append(bm.bots, bot)
}

func (bm *BotManager) Start() error {
	// Start request processor
	go bm.processRequests()

	// Start all bots
	for _, bot := range bm.bots {
		go func(b Bot) {
			if err := b.Start(bm.ctx); err != nil {
				log.Error().Err(err).Str("bot", b.GetBotName()).Msg("Bot failed to start")
			}
		}(bot)
	}

	log.Info().Int("bots", len(bm.bots)).Msg("Bot manager started")
	return nil
}

func (bm *BotManager) Stop() error {
	bm.cancel()

	for _, bot := range bm.bots {
		if err := bot.Stop(); err != nil {
			log.Error().Err(err).Str("bot", bot.GetBotName()).Msg("Error stopping bot")
		}
	}

	log.Info().Msg("Bot manager stopped")
	return nil
}

func (bm *BotManager) processRequests() {
	for {
		select {
		case <-bm.ctx.Done():
			return
		default:
			item := bm.requestQueue.Dequeue()
			if item != nil {
				go bm.handleRequest(*item)
			}
		}
	}
}

func (bm *BotManager) handleRequest(item QueueItem) {
	defer bm.requestQueue.MarkComplete()

	log.Info().Str("id", item.ID).Str("query", item.Query).Msg("Processing request")

	// Process the query using RAG
	response, err := bm.processQuery(bm.ctx, item.Query)
	if err != nil {
		log.Error().Err(err).Str("id", item.ID).Msg("Error processing query")
		item.Callback("Sorry, I encountered an error while processing your request.", err)
		return
	}

	item.Callback(response, nil)
	log.Info().Str("id", item.ID).Msg("Request completed")
}

func (bm *BotManager) processQuery(ctx context.Context, query string) (string, error) {
	// Search for relevant document chunks
	chunks, err := bm.rag.QueryDocumentChunks(ctx, query, 40)
	if err != nil {
		return "", fmt.Errorf("failed to query document chunks: %w", err)
	}

	// Rerank the chunks
	chunks, err = bm.rag.Rerank(ctx, query, chunks, 10)
	if err != nil {
		return "", fmt.Errorf("failed to rerank chunks: %w", err)
	}

	// Generate response using LLM
	askParam := &rag.AskParameter{
		Query: query,
		Limit: 10,
	}

	response, err := bm.rag.Ask(ctx, askParam)
	if err != nil {
		return "", fmt.Errorf("failed to generate response: %w", err)
	}

	return response, nil
}

func (bm *BotManager) EnqueueRequest(id, query, userID, platform string, callback func(string, error), queueNotifyCallback func(int)) {
	item := QueueItem{
		ID:        id,
		Query:     query,
		UserID:    userID,
		Platform:  platform,
		Callback:  callback,
		CreatedAt: time.Now(),
	}

	position := bm.requestQueue.Enqueue(item)

	// Notify user about their position in queue if callback is provided
	if queueNotifyCallback != nil {
		queueNotifyCallback(position)
	}
}

func (bm *BotManager) GetStatus() (queueLength int, activeJobs int) {
	return bm.requestQueue.GetQueueStatus()
}

// extractMention extracts the actual query from a message that mentions the bot
func extractMention(message, botName string) (string, bool) {
	// Remove @botname mentions and clean up the query
	mentions := []string{
		"@" + botName,
		"@" + strings.ToLower(botName),
	}

	query := message
	for _, mention := range mentions {
		query = strings.ReplaceAll(query, mention, "")
	}

	query = strings.TrimSpace(query)

	// Check if the original message contained a mention
	for _, mention := range mentions {
		if strings.Contains(message, mention) {
			return query, true
		}
	}

	return query, false
}

var botCmd = &cli.Command{
	Name:  "bot",
	Usage: "Start chat bots for Telegram and Slack",
	Flags: []cli.Flag{
		flagDSN,
		flagEmbeddingBaseURL,
		flagEmbeddingModel,
		flagAssistantBaseURL,
		flagAssistantModel,
		&cli.StringFlag{
			Name:    "telegram-token",
			Usage:   "Telegram bot token",
			Sources: cli.EnvVars("TELEGRAM_BOT_TOKEN"),
		},
		&cli.StringFlag{
			Name:    "slack-token",
			Usage:   "Slack bot token",
			Sources: cli.EnvVars("SLACK_BOT_TOKEN"),
		},
		&cli.StringFlag{
			Name:    "slack-app-token",
			Usage:   "Slack app token for socket mode",
			Sources: cli.EnvVars("SLACK_APP_TOKEN"),
		},
		&cli.IntFlag{
			Name:  "max-workers",
			Usage: "Maximum number of concurrent LLM requests",
			Value: 3,
		},
	},
	Action: func(ctx context.Context, command *cli.Command) error {
		dsn := command.String("dsn")
		embeddingBaseURL := command.String("embedding-base-url")
		embeddingModel := command.String("embedding-model")
		assistantBaseURL := command.String("assistant-base-url")
		assistantModel := command.String("assistant-model")
		telegramToken := command.String("telegram-token")
		slackToken := command.String("slack-token")
		slackAppToken := command.String("slack-app-token")
		maxWorkers := command.Int("max-workers")

		if telegramToken == "" && slackToken == "" {
			return fmt.Errorf("at least one bot token (telegram or slack) must be provided")
		}

		db, err := rag.OpenDuckDB(dsn)
		if err != nil {
			return err
		}

		embeddingClient := openai.NewClient(option.WithBaseURL(embeddingBaseURL))
		assistantClient := openai.NewClient(option.WithBaseURL(assistantBaseURL))
		r := &rag.RAG{
			DB:              db,
			EmbeddingClient: &embeddingClient,
			EmbeddingModel:  embeddingModel,
			AssistantClient: &assistantClient,
			AssistantModel:  assistantModel,
		}

		botManager := NewBotManager(r, maxWorkers)

		// Add Telegram bot if token is provided
		if telegramToken != "" {
			telegramBot := NewTelegramBot(telegramToken, botManager)
			botManager.AddBot(telegramBot)
			log.Info().Msg("Telegram bot configured")
		}

		// Add Slack bot if tokens are provided
		if slackToken != "" {
			slackBot := NewSlackBot(slackToken, slackAppToken, botManager)
			botManager.AddBot(slackBot)
			log.Info().Msg("Slack bot configured")
		}

		// Start bot manager
		err = botManager.Start()
		if err != nil {
			return err
		}

		// Wait for context cancellation
		<-ctx.Done()

		// Stop bot manager
		return botManager.Stop()
	},
}
