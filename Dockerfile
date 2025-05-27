FROM golang:1.20-alpine AS builder

WORKDIR /app

# Copy only necessary files and directories
COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
COPY ssl/ ./ssl/
COPY server/ ./server/
COPY metrics/ ./metrics/

# Build the binary
RUN go build -o ssl-exporter main.go

FROM alpine:latest

WORKDIR /app

# Copy the compiled binary
COPY --from=builder /app/ssl-exporter .

# Copy config files
COPY configs/ ./configs/

# Expose metrics port
EXPOSE 9115

CMD ["./ssl-exporter"]
