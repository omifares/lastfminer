# --- Compilation ---
FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o lastfm-miner main.go

# --- Container ---
FROM alpine:latest

# Dependencies
# - ffmpeg: Audio convertion
# - nodejs: JS Runtime for yt-dlp (SABR)
# - python3: Needed by yt-dlp
# - py-mutagem: Used to chage mp3 cover
# - curl: To download yt-dlp
RUN apk add --no-cache ffmpeg nodejs python3 py3-mutagen curl ca-certificates && \
    curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp && \
    chmod a+rx /usr/local/bin/yt-dlp

WORKDIR /app
COPY --from=builder /app/lastfm-miner .
COPY templates/ ./templates/

# YT-DLP Configs
RUN mkdir -p /root/.config/yt-dlp && \
    echo "--js-runtimes node" >> /root/.config/yt-dlp/config && \
    echo "--remote-components ejs:github" >> /root/.config/yt-dlp/config

EXPOSE 8080

CMD ["./lastfm-miner"]
