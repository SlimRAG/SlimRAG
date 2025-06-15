# Notes

## Create a new user and database in PostgreSQL

```postgresql
ALTER ROLE gorm SUPERUSER;

CREATE DATABASE ragtest OWNER gorm;
GRANT ALL PRIVILEGES ON DATABASE ragtest TO gorm;
```
