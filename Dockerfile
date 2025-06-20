FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application using make
ARG VERSION
ARG COMMIT_HASH
RUN make build

FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S rollytics && \
    adduser -u 1001 -S rollytics -G rollytics

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/rollytics .

# Change ownership to non-root user
RUN chown -R rollytics:rollytics /app

# Switch to non-root user
USER rollytics

# Expose port (adjust if needed based on your API configuration)
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["./rollytics"]

# Default command
CMD ["help"] 
