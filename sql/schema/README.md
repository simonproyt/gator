Goose migrations for Gator

Install Goose:

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -version
```

Run migrations (example):

```bash
cd sql/schema
goose postgres "postgres://username:password@host:5432/gator?sslmode=disable" up
goose postgres "postgres://username:password@host:5432/gator?sslmode=disable" down
```

Migration files live in this directory and must follow the naming format:

```
001_description.sql
002_another.sql
```

Each migration must contain the Goose markers (case sensitive):

-- +goose Up
-- +goose Down

Example connection string for local development (add `?sslmode=disable` in the config file):

```
postgres://postgres:postgres@localhost:5432/gator?sslmode=disable
```

Notes:
- Use `psql "<connection_string>"` to verify the database connection.
- Update your home config at `~/.gatorconfig.json` with the `db_url` key.
