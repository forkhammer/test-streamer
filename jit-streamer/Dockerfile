FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o jit-streamer .

FROM alpine:3.19

# Install ffprobe for video info (optional)
RUN apk add --no-cache ffmpeg

WORKDIR /app
COPY --from=builder /app/jit-streamer .

ENV PORT=8080
ENV VIDEOS_PATH=/videos
ENV SEGMENT_DURATION=4

EXPOSE 8080

CMD ["./jit-streamer"]
