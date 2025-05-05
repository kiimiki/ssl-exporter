FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o ssl-exporter main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/ssl-exporter .
COPY domains.json .
EXPOSE 9115
CMD ["./ssl-exporter"]
