FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -o feeds ./cmd/server

FROM alpine:latest

RUN apk add --no-cache yt-dlp ffmpeg sqlite ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/feeds .

# Data volume for SQLite database
VOLUME /app/data

ENV DB_PATH=/app/data/feeds.db

EXPOSE 8080

CMD ["./feeds"]
