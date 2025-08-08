# SlimRAG Deployment Guide

This document provides detailed instructions for deploying the SlimRAG application.

## 1. Prerequisites

Before you begin, ensure you have the following:

- A server running a Linux distribution.
- Go 1.24 or later installed.
- PostgreSQL installed and running.
- The `pgvector` extension for PostgreSQL installed.
- Access to OpenAI-compatible APIs for embedding, reranking, and language models.

## 2. Database Setup

1.  **Install `pgvector`:**

    Follow the instructions on the [pgvector GitHub repository](https://github.com/pgvector/pgvector) to install the extension for your version of PostgreSQL.

2.  **Create a database and user:**

    ```sql
    CREATE DATABASE slimrag;
    CREATE USER slimrag_user WITH PASSWORD 'your-password';
    GRANT ALL PRIVILEGES ON DATABASE slimrag TO slimrag_user;
    ```

3.  **Enable the `pgvector` extension:**

    Connect to your new database and run the following command:

    ```sql
    CREATE EXTENSION IF NOT EXISTS vector;
    ```

## 3. Application Setup

1.  **Clone the repository:**

    ```bash
    git clone https://github.com/fanyang89/SlimRAG.git
    cd SlimRAG
    ```

2.  **Build the application:**

    ```bash
    go build -o srag cmd/srag/main.go
    ```

3.  **Create a `.env` file:**

    Create a `.env` file in the root of the project with the following content, replacing the placeholder values with your actual configuration:

    ```
    DSN="postgres://slimrag_user:your-password@localhost:5432/slimrag"
    EMBEDDING_BASE_URL="your-embedding-api-endpoint"
    EMBEDDING_MODEL="your-embedding-model"
    RERANKER_BASE_URL="your-reranker-api-endpoint"
    RERANKER_MODEL="your-reranker-model"
    ASSISTANT_BASE_URL="your-assistant-api-endpoint"
    ASSISTANT_MODEL="your-assistant-model"
    ```

## 4. Running the Application

You can run the SlimRAG server in two ways:

### Directly

```bash
./srag serve
```

### As a `systemd` Service

Using a process manager like `systemd` is recommended for production environments.

1.  **Create a service file:**

    Create a file named `slimrag.service` in `/etc/systemd/system/` with the following content. Make sure to replace `your-user`, `your-group`, and the paths with your actual values.

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
    RestartSec=3

    [Install]
    WantedBy=multi-user.target
    ```

2.  **Enable and start the service:**

    ```bash
    sudo systemctl daemon-reload
    sudo systemctl enable slimrag.service
    sudo systemctl start slimrag.service
    ```

3.  **Check the status of the service:**

    ```bash
    sudo systemctl status slimrag.service
    ```

## 5. Populating the Database

Once the application is running, you need to populate the database with your documents.

1.  **Scan your documents:**

    ```bash
    ./srag scan /path/to/your/documents --glob "*.md"
    ```

2.  **Compute embeddings:**

    ```bash
    ./srag compute
    ```

Your SlimRAG instance is now deployed and ready to answer questions.