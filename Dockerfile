# ---------- stage 1: build the Svelte frontend ----------
FROM node:22-alpine AS web
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# ---------- stage 2: build the Go server ----------
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/mrome ./cmd/mrome

# ---------- stage 3: runtime ----------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 mrome
WORKDIR /app
COPY --from=build /out/mrome /app/mrome
COPY --from=web /app/web/dist /app/web/dist
RUN mkdir -p /music /data && chown -R mrome:mrome /music /data
ENV MROME_ADDR=:4533 \
    MROME_DATA_DIR=/data \
    MROME_MUSIC_DIR=/music \
    MROME_WEB_DIR=/app/web/dist \
    MROME_SCAN_ON_START=true
EXPOSE 4533
VOLUME ["/music", "/data"]
USER mrome
ENTRYPOINT ["/app/mrome"]
