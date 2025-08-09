# SlimRAG

SlimRAG is a minimalist Retrieval-Augmented Generation (RAG) system. It provides a command-line interface (CLI) to build and query a RAG system with your own documents.

## Features

- **Document Processing**: Scan directories, chunk documents, and compute embeddings with advanced strategies
- **Ask Questions**: Use LLMs to answer questions based on your documents with vector search and reranking
- **Flexible Configuration**: Configure database connections and API endpoints for embedding and assistant models
- **Multiple Platforms**: Start web servers, Telegram bots, and Slack bots for different interaction modes
- **Audit Logging**: Trace API calls and debug RAG performance with detailed audit logs
- **GitHub Issues**: Automatically answer consultation questions on GitHub issues
- **Extensible**: Built with a modular design in Go, making it easy to extend and customize

## Getting Started

### Prerequisites

- Go 1.24 or later
- **Database**: DuckDB for local development (PostgreSQL support was removed)
- Access to OpenAI-compatible APIs for embedding and language models

### Installation

1.  **Clone the repository:**

    ```bash
    git clone https://github.com/fanyang89/SlimRAG.git
    cd SlimRAG
    ```

2.  **Install dependencies:**

    ```bash
    go mod tidy
    ```

3.  **Build the `srag` CLI:**

    ```bash
    go build -o srag cmd/srag/main.go
    ```

### Configuration

SlimRAG is configured using environment variables or command-line flags. Create a `.env` file in the root of the project:

```
# Database Configuration
RAG_DSN="slimrag.db"  # DuckDB database file

# API Configuration
RAG_EMBEDDING_BASE_URL="your-embedding-api-endpoint"
RAG_EMBEDDING_MODEL="your-embedding-model"
RAG_ASSISTANT_BASE_URL="your-assistant-api-endpoint"
RAG_ASSISTANT_MODEL="your-assistant-model"
RAG_ASSISTANT_API_KEY="your-assistant-api-key"

# Bot Configuration (optional)
TELEGRAM_BOT_TOKEN="your-telegram-bot-token"
SLACK_BOT_TOKEN="your-slack-bot-token"
SLACK_APP_TOKEN="your-slack-app-token"

# GitHub Configuration (optional)
GITHUB_TOKEN="your-github-token"
```

## Usage

The `srag` CLI provides several commands to manage the RAG system:

### `ask` - Ask Questions

Ask questions using the RAG system with vector search and LLM processing.

```bash
# Basic query
./srag ask "What is SlimRAG?"

# Query from file (supports .txt, .ndjson, .jsonl)
./srag ask queries.txt

# Vector-only mode (no LLM processing)
./srag ask "What is SlimRAG?" --vector-only

# Custom system prompt
./srag ask "What is SlimRAG?" --system-prompt custom_prompt.txt

# Enable audit logging
./srag ask "What is SlimRAG?" --trace --audit-log-dir ./logs

# Configure retrieval limits
./srag ask "What is SlimRAG?" --retrieval-limit 50 --selected-limit 15
```

### `update` - Process Documents

Update documents with chunking and embedding computation.

```bash
# Basic update with adaptive chunking
./srag update ./docs --strategy adaptive --max-size 1000

# Configure workers and force reprocessing
./srag update ./docs --workers 5 --force

# Use custom chunking configuration
./srag update ./docs --config chunker_config.json

# Configure chunking parameters
./srag update ./docs --strategy semantic --language en --overlap 100
```

### `chunk` - Document Chunking

Chunk documents using advanced strategies.

```bash
# Basic chunking with adaptive strategy
./srag chunk input.md --output output.json

# Configure chunking parameters
./srag chunk input.md --strategy semantic --max-size 1500 --min-size 200 --overlap 100

# Use configuration file
./srag chunk input.md --config chunker_config.json

# Specify language
./srag chunk input.md --language zh
```

Available strategies:
- **adaptive**: Adaptive chunking (default)
- **fixed**: Fixed-size chunks
- **semantic**: Semantic-aware chunking
- **sentence**: Sentence-based chunking

### `serve` - Web Server

Start an HTTP server for API access.

```bash
# Start server on default port 5000
./srag serve

# Start on custom port
./srag serve --bind ":8080"
```

### `bot` - Chat Bots

Start Telegram and Slack bots with rate limiting and queue management.

```bash
# Start both bots
./srag bot --telegram-token="token" --slack-token="token" --slack-app-token="app_token"

# Start only Telegram bot
./srag bot --telegram-token="token"

# Configure concurrent workers
./srag bot --max-workers=5 --telegram-token="token"
```

### `issue-bot` - GitHub Issues Processing

Automatically answer consultation questions on GitHub issues.

```bash
# Process issues from a repository
./srag issue-bot --repo "owner/repo" --limit 10

# Configure RAG parameters
./srag issue-bot --repo "owner/repo" --rag-limit 50 --processed-file processed.json
```

### `health` - Service Health Check

Check database and API connectivity.

```bash
./srag health
```

### `get` - Retrieve Document Chunks

Get specific document chunks by ID.

```bash
./srag get chunk_id
```

## Quick Start with Docker

```bash
# Clone the repository
git clone https://github.com/fanyang89/SlimRAG.git
cd SlimRAG

# Start with DuckDB
docker-compose -f docker-compose.duckdb.yml up -d

# Start with bot support
docker-compose -f docker-compose.bot.yml up -d
```

## Deployment

### Database Options

SlimRAG uses DuckDB as its primary database backend:

- **DuckDB**: Embedded database with vector search capabilities
- No additional setup required
- Automatic VSS extension installation
- Single file database for easy deployment

### Deployment Steps

1.  **Build the application:**
    - Clone the repository on your server
    - Build the `srag` binary as described in the Installation section

2.  **Configure the environment:**
    - Create a `.env` file with your database and API configurations
    - Set up your embedding and assistant model endpoints

3.  **Run the application:**
    - Use `srag serve` for web API access
    - Use `srag bot` for chat bot functionality
    - Configure with process managers like `systemd` for production deployment

### Example `systemd` service file:

```ini
[Unit]
Description=SlimRAG Server
After=network.target

[Service]
User=your-user
Group=your-group
WorkingDirectory=/path/to/SlimRAG
ExecStart=/path/to/SlimRAG/srag serve
Restart=always

[Install]
WantedBy=multi-user.target
```

## Advanced Features

### Audit Logging
Enable detailed API call tracing and performance monitoring:
```bash
./srag ask "query" --trace --audit-log-dir ./audit_logs
```

### Batch Processing
Process multiple queries from files:
```bash
# Text file with one query per line
./srag ask queries.txt

# NDJSON/JSONL format
./srag ask queries.ndjson --jobs 4
```

### Document Management
The `update` command provides intelligent document processing:
- Automatic file change detection using hashes
- Cleanup of deleted files from database
- Configurable chunking strategies
- Parallel embedding computation

### Bot Features
- **Rate limiting**: Configurable request queues
- **Multi-platform**: Telegram and Slack support
- **Mention detection**: Responds to @mentions in groups
- **Error handling**: Graceful failure recovery

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue.

For detailed documentation on specific features, see the `docs/` directory.
