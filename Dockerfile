# =========================
# Build stage
# =========================
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git (needed for go mod sometimes)
RUN apk add --no-cache git

# Copy go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source tree
COPY . .

# Build the binary from cmd/main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o polybalance ./cmd

# =========================
# Runtime stage
# =========================
FROM gcr.io/distroless/base-debian12

WORKDIR /app

# Copy only the binary
COPY --from=builder /app/polybalance /app/polybalance

# Expose ports
EXPOSE 8080
EXPOSE 9090

# Run as non-root (best practice)
USER nonroot:nonroot

ENTRYPOINT ["/app/polybalance"]
