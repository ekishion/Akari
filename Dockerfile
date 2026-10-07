# Stage 1: Build Web Frontend SPA
FROM node:20-alpine AS web-builder

WORKDIR /app/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# Stage 2: Build Go Backend
FROM golang:alpine AS go-builder

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web-builder /app/web/dist ./web/dist

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o akari ./cmd/bridge

# Stage 3: Minimal Production Runtime
FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=go-builder /app/akari /app/akari

# Create data directory
RUN mkdir -p /data
ENV DATA_DIR=/data \
    HTTP_PORT=8096 \
    UDP_PORT=7359

# HTTP API & Video Stream
EXPOSE 8096
# UDP LAN Discovery
EXPOSE 7359/udp

VOLUME ["/data"]

ENTRYPOINT ["/app/akari"]
