# ── Stage 1: Builder ─────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

# Install C compiler — required for go-sqlite3 (cgo)
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy dependency files first — Docker layer cache
# If go.mod/go.sum unchanged, this layer is reused on rebuild
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build the server binary — static, no external dependencies
ARG CMD=cmd/server/main.go
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-w -s" \
    -o /app/bin \
    ./${CMD}

# ── Stage 2: Runtime ─────────────────────────────────────────────────
FROM alpine:3.19

# ca-certificates — required for HTTPS calls to MAL API
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin .

EXPOSE 8080 9090

CMD ["./bin"]