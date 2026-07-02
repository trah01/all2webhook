# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o all2webhook .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy from builder
COPY --from=builder /app/all2webhook /app/all2webhook

# Expose the dashboard and public inbound ports
EXPOSE 8080
EXPOSE 8081

# Run
CMD ["./all2webhook"]
