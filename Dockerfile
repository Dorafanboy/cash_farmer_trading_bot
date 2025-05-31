# Use the official Golang image as a builder image.
FROM golang:1.24-alpine AS builder

# Set the working directory inside the container.
WORKDIR /app

# Copy go module files and download dependencies first.
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire source code.
COPY . .

# Build the Go application.
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /bot ./cmd/bot/main.go

# Use a minimal base image like alpine for the final stage.
FROM alpine:latest

# Set the working directory.
WORKDIR /app

# Add a non-root user and switch to it.
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Copy the compiled binary from the builder stage.
COPY --from=builder /bot /app/bot

# Copy the example configuration file into a config directory.
COPY config/config.yml /app/config/config.yml

# Expose the port the application listens on.
EXPOSE 8080

# Define the command to run the application.
CMD ["/app/bot", "-config=/app/config/config.yml"] 