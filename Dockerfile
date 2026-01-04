# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /src

# Install git for go mod download
RUN apk add --no-cache git

# Set GOTOOLCHAIN to auto to allow automatic toolchain download
ENV GOTOOLCHAIN=auto

COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Run tests
RUN go test ./... -v

# Compile the binary
RUN go build -o /app/server ./cmd/server

# Production stage - final image
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

EXPOSE 8080/tcp 8081/tcp

COPY --from=builder /app/server /app/server
COPY --from=builder /src/migrations /app/migrations

CMD ["/app/server"]