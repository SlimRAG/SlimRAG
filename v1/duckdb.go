package rag

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/marcboeker/go-duckdb/v2"
	"github.com/rs/zerolog/log"
)

func OpenDuckDB(dsn string) (*sql.DB, error) {
	if len(dsn) == 0 {
		dsn = ":memory:"
	}

	connector, err := duckdb.NewConnector(dsn, nil)
	if err != nil {
		return nil, err
	}

	db := sql.OpenDB(connector)
	err = migrateDuckDB(db, dsn)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func migrateDuckDB(db *sql.DB, dsn string) error {
	// https://duckdb.org/docs/stable/core_extensions/vss.html#limitations
	_, err := db.Exec(`INSTALL vss; LOAD vss; SET hnsw_enable_experimental_persistence = true;`)
	if err != nil {
		return errors.Wrap(err, "Failed to install or load vss extension")
	}

	// Create metadata table to store embedding dimensions
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS rag_metadata (
			key VARCHAR PRIMARY KEY,
			value VARCHAR NOT NULL
		);
	`)
	if err != nil {
		return errors.Wrap(err, "Failed to create rag_metadata table")
	}

	// Check if embedding dimension is already set
	var storedDimension string
	err = db.QueryRow("SELECT value FROM rag_metadata WHERE key = 'embedding_dimension'").Scan(&storedDimension)
	if err != nil && err != sql.ErrNoRows {
		return errors.Wrap(err, "Failed to query embedding dimension")
	}

	// Only create document_chunks table if dimension is already set
	if storedDimension != "" && storedDimension != "0" {
		_, err = db.Exec(fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS document_chunks (
				id VARCHAR PRIMARY KEY,
				document_id VARCHAR,
				file_path VARCHAR,
				text VARCHAR,
				start_offset INTEGER,
				end_offset INTEGER,
				embedding FLOAT[%s]
			);
		`, storedDimension))
		if err != nil {
			return errors.Wrap(err, "Failed to create document_chunks table")
		}
	}

	// Add file_path column if it doesn't exist (only if table exists)
	if storedDimension != "" && storedDimension != "0" {
		_, err = db.Exec(`
			ALTER TABLE document_chunks ADD COLUMN IF NOT EXISTS file_path VARCHAR;
		`)
		if err != nil {
			return errors.Wrap(err, "Failed to add file_path column")
		}
	}

	// Create table to track processed files and their hashes
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS processed_files (
			file_path VARCHAR PRIMARY KEY,
			file_hash VARCHAR NOT NULL,
			processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return errors.Wrap(err, "Failed to create processed_files table")
	}

	// Only create HNSW index for in-memory databases (only if table exists)
	if dsn == ":memory:" && storedDimension != "" && storedDimension != "0" {
		_, err = db.Exec(`
			CREATE INDEX IF NOT EXISTS hnsw_idx ON document_chunks USING HNSW (embedding);
		`)
		if err != nil {
			return errors.Wrap(err, "Failed to create HNSW index")
		}
	}

	return nil
}

// GetStoredEmbeddingDimension retrieves the stored embedding dimension from the database
func GetStoredEmbeddingDimension(db *sql.DB, defaultDimension int64) int64 {
	var storedDimension string
	err := db.QueryRow("SELECT value FROM rag_metadata WHERE key = 'embedding_dimension'").
		Scan(&storedDimension)
	if err != nil {
		return defaultDimension
	}

	d, err := strconv.ParseInt(storedDimension, 10, 64)
	if err != nil {
		log.Panic().Err(err).Msg("")
	}

	if d != defaultDimension {
		log.Warn().
			Int64("stored_dimension", d).
			Int64("default_dimension", defaultDimension).
			Msg("Stored embedding dimension does not match default value, using stored value")
	}
	return d
}

// SetEmbeddingDimension sets the embedding dimension in the database
func SetEmbeddingDimension(db *sql.DB, dimension int64) error {
	// Check if dimension is already set
	var storedDimension string
	err := db.QueryRow("SELECT value FROM rag_metadata WHERE key = 'embedding_dimension'").Scan(&storedDimension)
	if err != nil && err != sql.ErrNoRows {
		return errors.Wrap(err, "Failed to query existing embedding dimension")
	}

	// If dimension is already set, don't allow changes
	if storedDimension != "" {
		var existingDim int64
		_, err = fmt.Sscanf(storedDimension, "%d", &existingDim)
		if err != nil {
			return errors.Wrap(err, "Failed to parse existing embedding dimension")
		}
		if existingDim != dimension {
			return errors.New("Embedding dimension cannot be changed after initial setup")
		}
		return nil
	}

	// Set the dimension in metadata
	_, err = db.Exec("INSERT INTO rag_metadata (key, value) VALUES ('embedding_dimension', ?)", fmt.Sprintf("%d", dimension))
	if err != nil {
		return errors.Wrap(err, "Failed to store embedding dimension")
	}

	// Rebuild table with new dimension (only works if no embeddings exist yet)
	err = RebuildTableWithNewDimension(db, dimension)
	if err != nil {
		return errors.Wrap(err, "Failed to rebuild table with new dimension")
	}

	return nil
}

// ValidateEmbeddingDimension checks if the provided dimension matches the stored dimension
func ValidateEmbeddingDimension(db *sql.DB, dimension int64) error {
	storedDimension, err := GetStoredEmbeddingDimension(db)
	if err != nil {
		return err
	}

	// If no dimension is set yet, this is valid
	if storedDimension == 0 {
		return nil
	}

	if storedDimension != dimension {
		return fmt.Errorf("Embedding dimension mismatch: provided %d, stored %d", dimension, storedDimension)
	}

	return nil
}

// RebuildTableWithNewDimension rebuilds the document_chunks table with a new embedding dimension
func RebuildTableWithNewDimension(db *sql.DB, newDimension int64) error {
	// Check if document_chunks table exists and has existing embeddings
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'document_chunks'").Scan(&count)
	if err != nil {
		return errors.Wrap(err, "Failed to check if document_chunks table exists")
	}

	if count > 0 {
		// Table exists, check for existing embeddings
		var embeddingCount int
		err := db.QueryRow("SELECT COUNT(*) FROM document_chunks WHERE embedding IS NOT NULL").Scan(&embeddingCount)
		if err != nil {
			return errors.Wrap(err, "Failed to count existing embeddings")
		}

		if embeddingCount > 0 {
			return fmt.Errorf("Cannot change embedding dimension: %d chunks already have embeddings", embeddingCount)
		}
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return errors.Wrap(err, "Failed to begin transaction")
	}
	defer func() { _ = tx.Rollback() }()

	// Drop existing backup table if it exists
	_, err = tx.Exec("DROP TABLE IF EXISTS document_chunks_backup")
	if err != nil {
		return errors.Wrap(err, "Failed to drop backup table")
	}

	// Only backup if table exists
	if count > 0 {
		// Rename current table to backup
		_, err = tx.Exec("ALTER TABLE document_chunks RENAME TO document_chunks_backup")
		if err != nil {
			return errors.Wrap(err, "Failed to backup existing table")
		}
	}

	// Create new table with new dimension
	_, err = tx.Exec(fmt.Sprintf(`
		CREATE TABLE document_chunks (
			id VARCHAR PRIMARY KEY,
			document_id VARCHAR,
			file_path VARCHAR,
			text VARCHAR,
			start_offset INTEGER,
			end_offset INTEGER,
			embedding FLOAT[%d]
		);
	`, newDimension))
	if err != nil {
		return errors.Wrap(err, "Failed to create new table")
	}

	// Copy data (without embeddings) only if backup table exists
	if count > 0 {
		_, err = tx.Exec(`
			INSERT INTO document_chunks (id, document_id, file_path, text, start_offset, end_offset)
			SELECT id, document_id, file_path, text, start_offset, end_offset
			FROM document_chunks_backup
		`)
		if err != nil {
			return errors.Wrap(err, "Failed to copy data to new table")
		}

		// Drop backup table
		_, err = tx.Exec("DROP TABLE document_chunks_backup")
		if err != nil {
			return errors.Wrap(err, "Failed to drop backup table")
		}
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		return errors.Wrap(err, "Failed to commit transaction")
	}

	return nil
}
