package rag

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/marcboeker/go-duckdb/v2"
	"github.com/rs/zerolog/log"
)

func OpenDuckDB(dsn string, defaultDimension int64) (*sql.DB, error) {
	if len(dsn) == 0 {
		dsn = ":memory:"
	}

	connector, err := duckdb.NewConnector(dsn, nil)
	if err != nil {
		return nil, err
	}

	db := sql.OpenDB(connector)
	err = MigrateDuckDB(db, defaultDimension)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func MigrateDuckDB(db *sql.DB, defaultDimension int64) error {
	// https://duckdb.org/docs/stable/core_extensions/vss.html#limitations
	_, err := db.Exec(`INSTALL vss; LOAD vss; SET hnsw_enable_experimental_persistence = true;`)
	if err != nil {
		return errors.Wrap(err, "Failed to install or load vss extension")
	}

	// Create metadata table to store embedding dimensions
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS meta (
			key VARCHAR PRIMARY KEY,
			value VARCHAR NOT NULL
		);
	`)
	if err != nil {
		return errors.Wrap(err, "Failed to create meta table")
	}

	// Check if embedding dimension is already set
	var storedDimension string
	err = db.QueryRow("SELECT value FROM meta WHERE key = 'embedding_dimension'").Scan(&storedDimension)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return errors.Wrap(err, "Failed to query embedding dimension")
	}
	if storedDimension == "" {
		storedDimension = fmt.Sprintf("%d", defaultDimension)
	}

	createTableDocumentChunks := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS document_chunks (
				id VARCHAR PRIMARY KEY,
				document_id VARCHAR,
				text VARCHAR,
				embedding FLOAT[%s]);`, storedDimension)
	_, err = db.Exec(createTableDocumentChunks)
	if err != nil {
		return errors.Wrap(err, "Failed to create document_chunks table")
	}

	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS hnsw_idx ON document_chunks USING HNSW (embedding);`)
	if err != nil {
		return errors.Wrap(err, "Failed to create hnsw_idx index")
	}

	_, err = db.Exec(`INSERT INTO meta (key, value) VALUES ('embedding_dimension', ?)`, storedDimension)
	if err != nil {
		return errors.Wrap(err, "Failed to insert metadata")
	}

	// Create table to track processed files and their hashes
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS processed_files (
			file_path VARCHAR PRIMARY KEY,
			file_name VARCHAR NOT NULL,
			file_hash VARCHAR NOT NULL,
			processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)
	if err != nil {
		return errors.Wrap(err, "Failed to create processed_files table")
	}

	return nil
}

// GetStoredEmbeddingDimension retrieves the stored embedding dimension from the database
func GetStoredEmbeddingDimension(db *sql.DB, defaultDimension int64) int64 {
	var storedDimension string
	err := db.QueryRow("SELECT value FROM meta WHERE key = 'embedding_dimension'").
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
