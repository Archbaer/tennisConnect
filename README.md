# Tennis Connect

## Local backend

Requirements: Go 1.25, Docker, and Docker Compose.

Start PostgreSQL/PostGIS:

```sh
docker compose up -d database
```

Configure and run the API:

```sh
export DATABASE_URL='postgres://tennisconnect:local-development-only@localhost:5432/tennisconnect?sslmode=disable'
go run .
```

The credentials above are for the local Docker database only. Production must use TLS and secret-managed credentials.

Check database-backed health:

```sh
curl --fail http://localhost:8080/healthz
```

Run checks:

```sh
gofmt -w main.go main_test.go
go test ./...
go vet ./...
```

`PORT` is optional and defaults to `8080`. The server refuses invalid configuration or an unavailable database at startup.
