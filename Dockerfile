FROM golang:1.20-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download
FROM golang:1.20-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY main.go ./
COPY ssl/ ./ssl/
COPY server/ ./server/
COPY metrics/ ./metrics/

# Явно подтягиваем все внешние зависимости
RUN go get go.mongodb.org/mongo-driver/mongo@v1.13.1
RUN go get github.com/prometheus/client_golang/prometheus/promhttp@v1.16.0

RUN go build -o ssl-exporter main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/ssl-exporter .
COPY configs/ ./configs/

EXPOSE 9115

CMD ["./ssl-exporter"]
