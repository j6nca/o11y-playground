# Start with a builder image to compile the Go application
FROM golang:1.24 AS builder

WORKDIR /app

# Copy the Go application source code
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the Go application binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /store-client

# Use a minimal image for the final container
FROM alpine:latest
WORKDIR /

# Copy the compiled binary from the builder stage
COPY --from=builder /store-client .

# Set the entry point to run the application
CMD ["/store-client"]
