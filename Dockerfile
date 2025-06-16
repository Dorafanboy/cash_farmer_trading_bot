# Use the official Golang image as a builder image.
FROM golang:1.24-alpine AS builder

# Install git (might be needed for some Go modules)
RUN apk add --no-cache git

# Set the working directory inside the container.
WORKDIR /app

# Copy go module files and download dependencies first for better caching.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy the entire source code.
COPY . .

# Build the Go application.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -a -installsuffix cgo -o /bot ./cmd/bot/main.go

# Use a minimal base image like alpine for the final stage.
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata && \
    update-ca-certificates

# Set the working directory.
WORKDIR /app

# Add a non-root user and switch to it.
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy the compiled binary from the builder stage.
COPY --from=builder /bot /app/bot

# Change ownership to the non-root user
RUN chown appuser:appgroup /app/bot
USER appuser

# Expose the port the application listens on.
EXPOSE 8080

# Run the application
CMD ["./bot"]