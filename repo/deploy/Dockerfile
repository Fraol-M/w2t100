# =============================================================================
# PropertyOps Backend — Multi-Stage Dockerfile
# =============================================================================
# Stage 1: Build all Go binaries
# Stage 2: Minimal runtime image with non-root user
# =============================================================================

# ---- Builder Stage ----
FROM golang:1.25-alpine3.21 AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /bin/propertyops-api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /bin/propertyops-migrate ./cmd/migrate && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /bin/propertyops-healthcheck ./cmd/healthcheck

# ---- Runtime Stage ----
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata mysql-client && \
    addgroup -S propertyops && \
    adduser -S -G propertyops propertyops

COPY --from=builder /bin/propertyops-api /usr/local/bin/propertyops-api
COPY --from=builder /bin/propertyops-migrate /usr/local/bin/propertyops-migrate
COPY --from=builder /bin/propertyops-healthcheck /usr/local/bin/propertyops-healthcheck
COPY db/migrations /app/migrations

RUN mkdir -p /var/lib/propertyops/storage \
             /var/lib/propertyops/backups \
             /var/log/propertyops \
             /run/propertyops/keys && \
    chown -R propertyops:propertyops /var/lib/propertyops /var/log/propertyops /run/propertyops

USER propertyops
WORKDIR /app

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/usr/local/bin/propertyops-healthcheck"]

ENTRYPOINT ["/usr/local/bin/propertyops-api"]
