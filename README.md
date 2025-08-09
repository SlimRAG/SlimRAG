# SlimRAG

SlimRAG is a minimalist Retrieval-Augmented Generation (RAG) system. It provides a command-line interface (CLI) to build and query a RAG system with your own documents.

## Features

- **Scan Documents**: Recursively scan a directory for documents to be included in the RAG system.
- **Compute Embeddings**: Generate vector embeddings for your documents using a configurable embedding model.
- **Ask Questions**: Use a large language model (LLM) to answer questions based on the documents in your database.
- **Flexible Configuration**: Configure database connections and API endpoints for embedding, reranking, and assistant models.
- **Extensible**: Built with a modular design in Go, making it easy to extend and customize.

## Getting Started

### Prerequisites

- Go 1.24 or later
- **Database**: Either PostgreSQL with the [pgvector](https://github.com/pgvector/pgvector) extension installed, or DuckDB for local development
- Access to OpenAI-compatible APIs for embedding and language models.

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

SlimRAG is configured using command-line flags or a `.env` file. Create a `.env` file in the root of the project and add the following variables:

```
# Database Configuration (choose one)
DSN="postgres://user:password@host:port/database"  # PostgreSQL
# DSN="slimrag.db"  # DuckDB for local development

# API Configuration
EMBEDDING_BASE_URL="your-embedding-api-endpoint"
EMBEDDING_MODEL="your-embedding-model"
RERANKER_BASE_URL="your-reranker-api-endpoint"
RERANKER_MODEL="your-reranker-model"
ASSISTANT_BASE_URL="your-assistant-api-endpoint"
ASSISTANT_MODEL="your-assistant-model"

# OpenAI API Key (if using OpenAI models)
OPENAI_API_KEY="sk-your-openai-api-key-here"
```

## Usage

The `srag` CLI has several commands to manage the RAG system.

### `scan`

Scan a directory for documents and upsert them into the database.

```bash
./srag scan <path-to-documents> --glob "*.md"
```

This command will scan the specified path for files matching the glob pattern and add them to the database.

### `compute`

Compute embeddings for the documents in the database.

```bash
./srag compute
```

This command will generate embeddings for all documents that don't have them yet. You can use the `--force` flag to re-compute embeddings for all documents.

### `ask`

Ask a question to the RAG system.

```bash
./srag ask "What is SlimRAG?"
```

This command will:
1.  Generate an embedding for your query.
2.  Search the database for relevant document chunks.
3.  Use a reranker to improve the relevance of the search results.
4.  Pass the relevant document chunks and your query to an LLM to generate an answer.

### `chunk`

Chunk markdown files using the built-in Go native chunker.

```bash
./srag chunk <input-file> --output <output.json> --strategy adaptive --max-size 1000 --min-size 100 --overlap 50 --language auto
```

This command provides advanced document chunking with multiple strategies:
- **fixed**: Fixed-size chunks
- **semantic**: Semantic-aware chunking
- **sentence**: Sentence-based chunking
- **adaptive**: Adaptive chunking (default)

Supports both Chinese and English text processing.

### `update`

Update documents in the database with advanced chunking and processing.

```bash
./srag update <path-to-documents> --strategy adaptive --max-size 1000 --workers 3 --force
```

This command combines document scanning, chunking, and embedding computation in one step.

### `serve`

Start a web server that provides an API for interacting with the RAG system.

```bash
./srag serve
```

The server will be available at `http://localhost:5000` by default.

### `bot`

Start chat bots for Telegram and Slack platforms. Users can interact with the RAG system by mentioning the bot in group chats or sending direct messages.

```bash
# Start both Telegram and Slack bots
./srag bot --telegram-token="your_telegram_token" --slack-token="your_slack_token" --slack-app-token="your_slack_app_token"

# Start only Telegram bot
./srag bot --telegram-token="your_telegram_token"

# Configure maximum concurrent workers (default: 3)
./srag bot --max-workers=5 --telegram-token="your_telegram_token"
```

The bot command includes:
- **Rate limiting**: Built-in request queue with configurable maximum concurrent workers
- **Multi-platform support**: Works with both Telegram and Slack
- **Mention detection**: Responds to @mentions in group chats and all messages in direct chats
- **Error handling**: Graceful error handling with user-friendly messages

For detailed bot setup instructions, see [docs/bot.md](docs/bot.md).

## Quick Start with Docker

```bash
# Clone the repository
git clone https://github.com/fanyang89/SlimRAG.git
cd SlimRAG

# Option 1: Start with PostgreSQL (production)
docker-compose -f docker-compose.bot.yml up -d

# Option 2: Start with DuckDB (development)
docker-compose -f docker-compose.duckdb.yml up -d
```

## Deployment

SlimRAG supports two database backends for flexible deployment options.

### Database Options

#### PostgreSQL (Recommended for Production)

For production deployments, use PostgreSQL with pgvector:

1.  **Set up PostgreSQL:**
    - Install PostgreSQL and the `pgvector` extension
    - Create a database and user for SlimRAG
    - Enable the vector extension

#### DuckDB (Local Development)

For local development or single-user deployments:

1.  **Use DuckDB:**
    - No additional setup required
    - Set `DSN="slimrag.db"` in your `.env` file
    - Automatic VSS extension installation

### Deployment Steps

1.  **Build the application:**
    - Clone the repository on your server
    - Build the `srag` binary as described in the Installation section

2.  **Configure the environment:**
    - Create a `.env` file with your database connection details and API endpoints
    - Choose between PostgreSQL or DuckDB based on your needs

3.  **Run the application:**
    - You can run the `srag serve` command directly or use a process manager like `systemd` to run it as a service

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

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue.
