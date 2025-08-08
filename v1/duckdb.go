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
	err = migrateDuckDB(db)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func migrateDuckDB(db *sql.DB) error {
	_, err := db.Exec(`
		INSTALL vss;
		LOAD vss;
	`)
	if err != nil {
		return errors.Wrap(err, "Failed to install or load vss extension")
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS document_chunks (
			id VARCHAR PRIMARY KEY,
			document_id VARCHAR,
			text VARCHAR,
			start_offset INTEGER,
			end_offset INTEGER,
			embedding FLOAT[384]
		);
	`)
	if err != nil {
		return errors.Wrap(err, "Failed to create document_chunks table")
	}

	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS hnsw_idx ON document_chunks USING HNSW (embedding);
	`)
	if err != nil {
		return errors.Wrap(err, "Failed to create HNSW index")
	}

	return nil
}