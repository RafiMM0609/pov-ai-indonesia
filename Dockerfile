# Multi-stage Dockerfile for Go application
FROM golang:1.23-alpine AS builder

# Set working directory
WORKDIR /app

# Install ca-certificates and tzdata
RUN apk add --no-cache ca-certificates tzdata

# Copy go mod files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o pov-ai ./cmd/main.go

# Production stage using minimal alpine image
FROM alpine:3.20

# Install ca-certificates and tzdata for HTTPS requests and accurate timezones
RUN apk add --no-cache ca-certificates tzdata

# Set timezone to Asia/Jakarta (WIB)
ENV TZ=Asia/Jakarta

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/pov-ai /app/pov-ai

# Copy static frontend assets & templates
COPY web/ ./web/

# Create data directories
RUN mkdir -p ./data ./data/knowledge

# Expose HTTP port
EXPOSE 8080

# Run binary
CMD ["/app/pov-ai"]
