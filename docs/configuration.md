# Configuration Guide

SlimRAG provides flexible configuration options through environment variables, command-line flags, and configuration files.

## Environment Variables

SlimRAG can be configured using environment variables. Create a `.env` file in the project root or set these variables in your environment:

### Database Configuration

```bash
# Choose one database backend:

# PostgreSQL (recommended for production)
DSN="postgres://user:password@localhost:5432/slimrag?sslmode=disable"

# DuckDB (for local development)
DSN="slimrag.db"
# or for in-memory database:
# DSN=":memory:"
```

### API Configuration

```bash
# Embedding Model Configuration
EMBEDDING_BASE_URL="https://api.openai.com/v1"
EMBEDDING_MODEL="text-embedding-ada-002"

# Reranker Model Configuration
RERANKER_BASE_URL="https://api.openai.com/v1"
RERANKER_MODEL="gpt-3.5-turbo"

# Assistant Model Configuration
ASSISTANT_BASE_URL="https://api.openai.com/v1"
ASSISTANT_MODEL="gpt-3.5-turbo"

# OpenAI API Key (if using OpenAI models)
OPENAI_API_KEY="sk-your-openai-api-key-here"
```

### Bot Configuration (Optional)

```bash
# Telegram Bot
TELEGRAM_BOT_TOKEN="your-telegram-bot-token"

# Slack Bot
SLACK_BOT_TOKEN="your-slack-bot-token"
SLACK_APP_TOKEN="your-slack-app-token"
```

### Server Configuration

```bash
# Server Settings
SERVER_PORT="5000"
SERVER_HOST="0.0.0.0"

# Performance Settings
MAX_WORKERS="3"
RAG_LIMIT="10"
RAG_TOP_N="5"

# Logging
LOG_LEVEL="info"
```

## Chunking Configuration

SlimRAG supports advanced document chunking with configurable strategies. You can configure chunking through:

1. **Command-line flags** (for individual commands)
2. **Configuration files** (for reusable settings)

### Chunking Strategies

- **`fixed`**: Split documents into fixed-size chunks
- **`semantic`**: Split based on semantic boundaries
- **`sentence`**: Split at sentence boundaries
- **`adaptive`**: Dynamically adjust chunk size based on content

### Configuration File

Create a JSON configuration file (e.g., `configs/chunker_config.json`):

```json
{
  "max_chunk_size": 1000,
  "min_chunk_size": 100,
  "overlap_size": 50,
  "sentence_window": 3,
  "strategy": "adaptive",
  "language": "auto",
  "preserve_sections": true,
  "similarity_threshold": 0.7
}
```

#### Configuration Parameters

- **`max_chunk_size`**: Maximum number of characters per chunk
- **`min_chunk_size`**: Minimum number of characters per chunk
- **`overlap_size`**: Number of overlapping characters between chunks
- **`sentence_window`**: Number of sentences to consider for context
- **`strategy`**: Chunking strategy (`fixed`, `semantic`, `sentence`, `adaptive`)
- **`language`**: Document language (`zh`, `en`, `auto`)
- **`preserve_sections`**: Whether to preserve document section boundaries
- **`similarity_threshold`**: Threshold for semantic similarity (0.0-1.0)

### Using Configuration Files

```bash
# Use with chunk command
./srag chunk document.md --config configs/chunker_config.json

# Use with update command
./srag update /path/to/docs --config configs/chunker_config.json
```

### Command-line Override

You can override configuration file settings with command-line flags:

```bash
# Override strategy and max size
./srag update /path/to/docs --config configs/chunker_config.json --strategy semantic --max-size 1500
```

## Database Backends

### PostgreSQL Setup

1. **Install PostgreSQL and pgvector:**
   ```bash
   # Ubuntu/Debian
   sudo apt-get install postgresql postgresql-contrib
   
   # Install pgvector extension
   # Follow instructions at: https://github.com/pgvector/pgvector
   ```

2. **Create database:**
   ```sql
   CREATE DATABASE slimrag;
   CREATE USER slimrag_user WITH PASSWORD 'your-password';
   GRANT ALL PRIVILEGES ON DATABASE slimrag TO slimrag_user;
   \c slimrag
   CREATE EXTENSION IF NOT EXISTS vector;
   ```

3. **Configure DSN:**
   ```bash
   DSN="postgres://slimrag_user:your-password@localhost:5432/slimrag?sslmode=disable"
   ```

### DuckDB Setup

1. **No installation required** - DuckDB is embedded
2. **Configure DSN:**
   ```bash
   # File-based database
   DSN="slimrag.db"
   
   # In-memory database (for testing)
   DSN=":memory:"
   ```

## Model Providers

### OpenAI

```bash
EMBEDDING_BASE_URL="https://api.openai.com/v1"
EMBEDDING_MODEL="text-embedding-ada-002"
ASSISTANT_BASE_URL="https://api.openai.com/v1"
ASSISTANT_MODEL="gpt-3.5-turbo"
OPENAI_API_KEY="sk-your-api-key"
```

### Custom OpenAI-Compatible APIs

```bash
# Example: Local Ollama instance
EMBEDDING_BASE_URL="http://localhost:11434/v1"
EMBEDDING_MODEL="nomic-embed-text"
ASSISTANT_BASE_URL="http://localhost:11434/v1"
ASSISTANT_MODEL="llama2"
```

## Performance Tuning

### Worker Configuration

```bash
# Adjust based on your system resources
MAX_WORKERS="3"  # Number of concurrent embedding computations
```

### RAG Parameters

```bash
RAG_LIMIT="10"   # Maximum number of chunks to retrieve
RAG_TOP_N="5"    # Number of top chunks to use for answer generation
```

### Chunking Performance

- **Large documents**: Use `adaptive` strategy with larger `max_chunk_size`
- **Small documents**: Use `sentence` strategy with smaller chunks
- **Mixed content**: Use `semantic` strategy for better context preservation

## Troubleshooting

### Common Issues

1. **Database connection errors:**
   - Verify DSN format and credentials
   - Ensure database server is running
   - Check network connectivity

2. **Embedding API errors:**
   - Verify API keys and endpoints
   - Check rate limits
   - Ensure model names are correct

3. **Performance issues:**
   - Reduce `MAX_WORKERS` if system is overloaded
   - Adjust chunk sizes for your content type
   - Use DuckDB for local development

### Debug Mode

```bash
# Enable debug logging
LOG_LEVEL="debug"

# Or use command-line flag
./srag ask "question" --log-level debug
```

## Security Considerations

1. **API Keys:**
   - Store API keys in environment variables, not in code
   - Use `.env` files for local development
   - Rotate keys regularly

2. **Database Security:**
   - Use strong passwords
   - Enable SSL/TLS for PostgreSQL connections
   - Restrict database access to necessary hosts

3. **Bot Tokens:**
   - Keep bot tokens secure
   - Use environment variables for production
   - Regenerate tokens if compromised