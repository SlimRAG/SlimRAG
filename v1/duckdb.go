package rag

import (
	"database/sql"

	"github.com/cockroachdb/errors"
	"github.com/marcboeker/go-duckdb/v2"
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

	// TODO: set dims from env
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS document_chunks (
			id VARCHAR PRIMARY KEY,
			document_id VARCHAR,
			file_path VARCHAR,
			text VARCHAR,
			start_offset INTEGER,
			end_offset INTEGER,
			embedding FLOAT[1024]
		);
	`)
	if err != nil {
		return errors.Wrap(err, "Failed to create document_chunks table")
	}

	// Add file_path column if it doesn't exist
	_, err = db.Exec(`
		ALTER TABLE document_chunks ADD COLUMN IF NOT EXISTS file_path VARCHAR;
	`)
	if err != nil {
		return errors.Wrap(err, "Failed to add file_path column")
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

	// Only create HNSW index for in-memory databases
	if dsn == ":memory:" {
		_, err = db.Exec(`
			CREATE INDEX IF NOT EXISTS hnsw_idx ON document_chunks USING HNSW (embedding);
		`)
		if err != nil {
			return errors.Wrap(err, "Failed to create HNSW index")
		}
	}

	return nil
}
