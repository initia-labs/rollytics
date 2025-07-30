FROM golang:1.24-alpine AS builder

# Set build info
ARG VERSION

ARG COMMIT_HASH

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

# Copy migration files
COPY --from=builder /app/orm/migrations ./orm/migrations

# Change ownership to non-root user
RUN chown -R rollytics:rollytics /app

# Install atlas
RUN wget https://release.ariga.io/atlas/atlas-linux-amd64-latest -O /usr/local/bin/atlas && \
    chmod +x /usr/local/bin/atlas

# Switch to non-root user
USER rollytics

# Expose port (adjust if needed based on your API configuration)
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["./rollytics"]

# Default command
CMD ["help"]
