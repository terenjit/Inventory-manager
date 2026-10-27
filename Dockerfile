# Build stage
FROM golang:1.26 AS builder
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server, seed, and checkledger binaries
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/bin/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/bin/seed ./cmd/seed && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/bin/checkledger ./cmd/checkledger

# Runtime stage
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binaries
COPY --from=builder /app/bin/server ./bin/server
COPY --from=builder /app/bin/seed ./bin/seed
COPY --from=builder /app/bin/checkledger ./bin/checkledger

# Copy web assets and templates (required by webui)
COPY --from=builder /app/web ./web

# Copy migrations (needed for golang-migrate or reference)
COPY --from=builder /app/migrations ./migrations

# Copy .env.example as reference (optional)
COPY --from=builder /app/.env.example ./.env.example

# Create non-root user
RUN addgroup -g 1000 appuser && adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

# Default port
EXPOSE 8080

# Environment variables can be passed at runtime
ENV APP_PORT=8080

# Run the server
CMD ["./bin/server"]
