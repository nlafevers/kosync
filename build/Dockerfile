# Build stage
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy go.mod and go.sum and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application as a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o kosync ./cmd/kosync/main.go

# Run stage
FROM alpine:3.22

# Add CA certificates for HTTPS requests if needed and tzdata
RUN apk add --no-cache ca-certificates tzdata

# Create a non-root user
RUN addgroup -S kosync && adduser -S -D -H -h /app -G kosync kosync

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/kosync .

# Create default directory for data
RUN mkdir -p /data && chown -R kosync:kosync /data

USER kosync

# Expose the default port
EXPOSE 8081

# Run the binary
ENTRYPOINT ["./kosync"]
