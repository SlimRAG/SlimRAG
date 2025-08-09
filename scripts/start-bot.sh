#!/bin/bash

# SlimRAG Bot Startup Script
# This script helps you start the SlimRAG chat bots with proper configuration

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if .env file exists
if [ ! -f ".env" ]; then
    print_warning ".env file not found. Creating from .env.example..."
    if [ -f ".env.example" ]; then
        cp .env.example .env
        print_info "Created .env file from .env.example"
        print_warning "Please edit .env file with your actual configuration before running the bot"
        exit 1
    else
        print_error ".env.example file not found. Please create .env file manually."
        exit 1
    fi
fi

# Load environment variables
source .env

# Check if srag binary exists
if [ ! -f "./srag" ]; then
    print_info "Building srag binary..."
    go build -o srag ./cmd/srag
    if [ $? -eq 0 ]; then
        print_info "Successfully built srag binary"
    else
        print_error "Failed to build srag binary"
        exit 1
    fi
fi

# Check required configuration
if [ -z "$TELEGRAM_BOT_TOKEN" ] && [ -z "$SLACK_BOT_TOKEN" ]; then
    print_error "At least one bot token (TELEGRAM_BOT_TOKEN or SLACK_BOT_TOKEN) must be configured in .env"
    exit 1
fi

if [ -z "$DSN" ]; then
    print_error "DSN must be configured in .env"
    exit 1
fi

if [ -z "$EMBEDDING_BASE_URL" ] || [ -z "$EMBEDDING_MODEL" ]; then
    print_error "EMBEDDING_BASE_URL and EMBEDDING_MODEL must be configured in .env"
    exit 1
fi

if [ -z "$ASSISTANT_BASE_URL" ] || [ -z "$ASSISTANT_MODEL" ]; then
    print_error "ASSISTANT_BASE_URL and ASSISTANT_MODEL must be configured in .env"
    exit 1
fi

# Build command arguments
CMD_ARGS=""

if [ -n "$TELEGRAM_BOT_TOKEN" ]; then
    CMD_ARGS="$CMD_ARGS --telegram-token=$TELEGRAM_BOT_TOKEN"
    print_info "Telegram bot will be started"
fi

if [ -n "$SLACK_BOT_TOKEN" ]; then
    CMD_ARGS="$CMD_ARGS --slack-token=$SLACK_BOT_TOKEN"
    if [ -n "$SLACK_APP_TOKEN" ]; then
        CMD_ARGS="$CMD_ARGS --slack-app-token=$SLACK_APP_TOKEN"
    fi
    print_info "Slack bot will be started"
fi

# Performance configuration
if [ -n "$MAX_WORKERS" ]; then
    CMD_ARGS="$CMD_ARGS --max-workers=$MAX_WORKERS"
fi

if [ -n "$RAG_LIMIT" ]; then
    CMD_ARGS="$CMD_ARGS --rag-limit=$RAG_LIMIT"
fi

if [ -n "$RAG_TOP_N" ]; then
    CMD_ARGS="$CMD_ARGS --rag-top-n=$RAG_TOP_N"
fi

# Optional chunking configuration overrides
if [ -n "$CHUNK_STRATEGY" ]; then
    CMD_ARGS="$CMD_ARGS --chunk-strategy=$CHUNK_STRATEGY"
fi

if [ -n "$CHUNK_SIZE" ]; then
    CMD_ARGS="$CMD_ARGS --chunk-size=$CHUNK_SIZE"
fi

if [ -n "$CHUNK_OVERLAP" ]; then
    CMD_ARGS="$CMD_ARGS --chunk-overlap=$CHUNK_OVERLAP"
fi

if [ -n "$CHUNK_LANGUAGE" ]; then
    CMD_ARGS="$CMD_ARGS --chunk-language=$CHUNK_LANGUAGE"
fi

# Start the bot
print_info "Starting SlimRAG bot with configuration:"
print_info "  DSN: $DSN"
print_info "  Embedding Model: $EMBEDDING_MODEL"
print_info "  Assistant Model: $ASSISTANT_MODEL"
print_info "  Max Workers: ${MAX_WORKERS:-3}"
if [ -n "$RAG_LIMIT" ]; then
    print_info "  RAG Limit: $RAG_LIMIT"
fi
if [ -n "$RAG_TOP_N" ]; then
    print_info "  RAG Top N: $RAG_TOP_N"
fi
if [ -n "$CHUNK_STRATEGY" ]; then
    print_info "  Chunk Strategy: $CHUNK_STRATEGY"
fi
if [ -n "$CHUNK_SIZE" ]; then
    print_info "  Chunk Size: $CHUNK_SIZE"
fi
if [ -n "$CHUNK_OVERLAP" ]; then
    print_info "  Chunk Overlap: $CHUNK_OVERLAP"
fi
if [ -n "$CHUNK_LANGUAGE" ]; then
    print_info "  Chunk Language: $CHUNK_LANGUAGE"
fi

print_info "Running command: ./srag bot$CMD_ARGS"
print_info "Press Ctrl+C to stop the bot"

exec ./srag bot$CMD_ARGS