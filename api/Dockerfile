FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for go mod download
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /agent-eval-api ./cmd/server

# Final stage
FROM alpine:3.19

# Install ca-certificates for HTTPS and wget for healthcheck
RUN apk add --no-cache ca-certificates wget

WORKDIR /app

# Copy the binary
COPY --from=builder /agent-eval-api /app/agent-eval-api

# Create data directory
RUN mkdir -p /data

EXPOSE 8080

ENTRYPOINT ["/app/agent-eval-api"]
