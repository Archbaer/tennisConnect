# Build the static SvelteKit site first so the Go image carries no Node runtime.
FROM node:24-bookworm-slim AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.26.11-bookworm AS api
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=api /out/api /app/api
COPY --from=web /app/web/build /app/web/build
ENV WEB_DIR=/app/web/build
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/api"]
