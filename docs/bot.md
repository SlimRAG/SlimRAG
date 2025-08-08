# Chat Bot Integration

SlimRAG supports chat bot integration for Telegram and Slack platforms. Users can interact with the RAG system by mentioning the bot in group chats or sending direct messages.

## Features

- **Multi-platform Support**: Works with both Telegram and Slack
- **Rate Limiting**: Built-in request queue with configurable maximum concurrent workers
- **Mention Detection**: Responds to @mentions in group chats and all messages in direct chats
- **Error Handling**: Graceful error handling with user-friendly error messages
- **Logging**: Comprehensive logging for monitoring and debugging

## Configuration

### Environment Variables

Set the following environment variables or use command-line flags:

```bash
# Telegram Bot Configuration
TELEGRAM_BOT_TOKEN="your_telegram_bot_token"

# Slack Bot Configuration
SLACK_BOT_TOKEN="xoxb-your-slack-bot-token"
SLACK_APP_TOKEN="xapp-your-slack-app-token"

# RAG Configuration (same as other commands)
DSN="your_database_connection_string"
EMBEDDING_BASE_URL="your_embedding_api_endpoint"
EMBEDDING_MODEL="your_embedding_model"
ASSISTANT_BASE_URL="your_assistant_api_endpoint"
ASSISTANT_MODEL="your_assistant_model"
```

### Command Line Usage

```bash
# Start both Telegram and Slack bots
./srag bot --telegram-token="your_token" --slack-token="your_token" --slack-app-token="your_app_token"

# Start only Telegram bot
./srag bot --telegram-token="your_token"

# Start only Slack bot
./srag bot --slack-token="your_token" --slack-app-token="your_app_token"

# Configure maximum concurrent workers (default: 3)
./srag bot --max-workers=5 --telegram-token="your_token"
```

## Bot Setup

### Telegram Bot Setup

1. **Create a Bot**:
   - Message @BotFather on Telegram
   - Use `/newbot` command
   - Follow the instructions to create your bot
   - Save the bot token

2. **Configure Bot**:
   - Set bot commands using `/setcommands` with BotFather
   - Configure privacy settings if needed

3. **Add to Groups** (optional):
   - Add the bot to your group chats
   - The bot will only respond when mentioned with @botname

### Slack Bot Setup

1. **Create a Slack App**:
   - Go to https://api.slack.com/apps
   - Click "Create New App"
   - Choose "From scratch"
   - Enter app name and select workspace

2. **Configure Bot Token**:
   - Go to "OAuth & Permissions"
   - Add the following Bot Token Scopes:
     - `app_mentions:read`
     - `channels:history`
     - `chat:write`
     - `im:history`
     - `im:read`
     - `im:write`
   - Install the app to your workspace
   - Copy the "Bot User OAuth Token"

3. **Enable Socket Mode**:
   - Go to "Socket Mode"
   - Enable Socket Mode
   - Create an App-Level Token with `connections:write` scope
   - Copy the App-Level Token

4. **Configure Event Subscriptions**:
   - Go to "Event Subscriptions"
   - Subscribe to the following bot events:
     - `app_mention`
     - `message.channels`
     - `message.im`

## Usage

### In Group Chats

Mention the bot with your question:

```
@SlimRAGBot How do I configure the embedding model?
```

### In Direct Messages

Send your question directly:

```
What are the best practices for document chunking?
```

### Rate Limiting

The bot implements a queue system to handle multiple requests:

- **Maximum Workers**: Configurable limit on concurrent LLM requests
- **Queue Management**: Requests are queued when all workers are busy
- **Fair Processing**: First-in-first-out request processing
- **Status Monitoring**: Queue length and active jobs are logged

## Architecture

### Components

1. **BotManager**: Central coordinator for all bot instances
2. **RequestQueue**: Rate limiting and queuing mechanism
3. **Bot Interface**: Common interface for different platforms
4. **TelegramBot**: Telegram-specific implementation
5. **SlackBot**: Slack-specific implementation

### Request Flow

1. User sends message mentioning the bot
2. Bot detects mention and extracts query
3. Request is queued with unique ID
4. Worker processes request using RAG system
5. Response is sent back to user
6. Request completion is logged

### Error Handling

- **Network Errors**: Automatic retry for transient failures
- **API Errors**: Graceful degradation with user notification
- **Queue Overflow**: Requests are queued indefinitely (consider adding timeout)
- **Invalid Queries**: Handled by the RAG system with appropriate responses

## Monitoring

### Logs

The bot generates structured logs for:

- Bot startup and shutdown events
- Request queuing and processing
- Error conditions and recovery
- Queue status and performance metrics

### Metrics

Monitor these key metrics:

- Queue length
- Active worker count
- Request processing time
- Error rates by platform
- User engagement statistics

## Security Considerations

1. **Token Security**: Store bot tokens securely, never commit to version control
2. **Rate Limiting**: Prevents abuse and protects backend resources
3. **Input Validation**: All user inputs are processed through the RAG system
4. **Error Messages**: Generic error messages prevent information leakage
5. **Logging**: Sensitive information is not logged

## Troubleshooting

### Common Issues

1. **Bot Not Responding**:
   - Check bot token validity
   - Verify network connectivity
   - Check bot permissions in chat

2. **Slow Responses**:
   - Monitor queue length
   - Increase max workers if needed
   - Check RAG system performance

3. **Telegram Issues**:
   - Verify bot token format
   - Check if bot is added to group
   - Ensure proper mention format (@botname)

4. **Slack Issues**:
   - Verify both bot token and app token
   - Check Socket Mode configuration
   - Ensure proper event subscriptions
   - Verify bot permissions in workspace

### Debug Mode

Enable debug logging by setting the log level:

```bash
LOG_LEVEL=debug ./srag bot --telegram-token="your_token"
```

## Limitations

1. **Simplified Implementation**: Current implementation uses placeholder HTTP clients
2. **No Persistence**: Queue state is not persisted across restarts
3. **Basic Error Recovery**: Limited retry mechanisms
4. **No User Authentication**: All users have equal access
5. **No Usage Quotas**: No per-user rate limiting

## Future Enhancements

1. **Full HTTP Client Implementation**: Complete Telegram and Slack API integration
2. **Persistent Queue**: Redis or database-backed request queue
3. **User Management**: Authentication and authorization
4. **Usage Analytics**: Detailed usage statistics and reporting
5. **Multi-language Support**: Internationalization for responses
6. **Rich Formatting**: Support for markdown, buttons, and attachments
7. **Conversation Context**: Multi-turn conversation support
8. **Admin Commands**: Bot management through chat interface