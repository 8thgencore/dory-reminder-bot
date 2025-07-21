# Use the official Golang image as the base for building the application
FROM golang:1.24.5-alpine3.21 AS builder

# Update and upgrade the Alpine packages, install build dependencies for CGO
RUN apk update && apk upgrade --available && \
    apk add --no-cache gcc musl-dev && \
    # Create a new user 'bot' with specific parameters
    adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "10001" \
    "bot"

WORKDIR /opt/app/

# Copy the go.mod and go.sum first to install dependencies
COPY go.mod go.sum ./

# Download the Go module dependencies and verify them
RUN go mod download && go mod verify

# Copy the entire application code into the working directory
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/

# Build the application with CGO enabled for sqlite3 support
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o ./bin/main cmd/bot/main.go

###########
# 2 stage #
###########
# Use a minimal base image to run the application
FROM golang:1.24.5-alpine3.21

# Install runtime dependencies for SQLite
RUN apk add --no-cache sqlite

# Set the working directory in the new image
WORKDIR /opt/app/

# Copy the passwd and group files from the builder stage for the user 'bot'
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy the compiled binary and configuration file from the builder stage
# Ensure the ownership is set to the 'bot' user and group
COPY --from=builder --chown=bot:bot /opt/app/bin/main .

# Create data directory and set proper ownership
RUN mkdir -p data && chown bot:bot data

# Set the user and group for running the application
USER bot:bot

# Command to run the application with the specified configuration file
ENTRYPOINT ["./main"]
