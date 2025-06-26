# Notes

## Create a new user and database in PostgreSQL

```postgresql
ALTER ROLE gorm SUPERUSER;

CREATE DATABASE ragtest OWNER gorm;
GRANT ALL PRIVILEGES ON DATABASE ragtest TO gorm;
```

## Load extensions

```postgresql
CREATE EXTENSION IF NOT EXISTS vector;
```

## Create index

```postgresql
SET maintenance_work_mem = '8GB';
SET max_parallel_maintenance_workers = 32;
CREATE INDEX ON document_chunks USING hnsw (embedding halfvec_l2_ops);
```
