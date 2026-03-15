# Multi-stage build for Go API services.
# Produces a single binary that can run as:
#   ./omnivore server api              — GraphQL/REST API
#   ./omnivore server backend-worker   — Background queue worker
#   ./omnivore server content-fetcher  — Content fetch worker (needs Chromium)
#
# Usage:
#   docker build --target api -t omnivore-api-go .
#   docker build --target backend-worker -t omnivore-backend-worker .
#   docker build --target content-fetcher -t omnivore-content-fetcher .

# ─── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS build
LABEL org.opencontainers.image.source="https://github.com/omnivore-app/omnivore"

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o omnivore .

# ─── API server ───────────────────────────────────────────────────────────────
FROM alpine:3.20 AS api
LABEL org.opencontainers.image.source="https://github.com/omnivore-app/omnivore"

RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=build /app/omnivore .

ENV PORT=8080
EXPOSE 8080

CMD ["./omnivore", "server", "api"]

# ─── Backend worker ───────────────────────────────────────────────────────────
FROM alpine:3.20 AS backend-worker
LABEL org.opencontainers.image.source="https://github.com/omnivore-app/omnivore"

RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=build /app/omnivore .

ENV PORT=8081
EXPOSE 8081

CMD ["./omnivore", "server", "backend-worker"]

# ─── Content fetcher (needs Chromium) ─────────────────────────────────────────
FROM alpine:3.20 AS content-fetcher
LABEL org.opencontainers.image.source="https://github.com/omnivore-app/omnivore"

RUN echo "@edge https://dl-cdn.alpinelinux.org/alpine/edge/community" >> /etc/apk/repositories \
 && echo "@edge https://dl-cdn.alpinelinux.org/alpine/edge/main" >> /etc/apk/repositories \
 && echo "@edge https://dl-cdn.alpinelinux.org/alpine/edge/testing" >> /etc/apk/repositories \
 && apk -U upgrade \
 && apk add --no-cache \
      chromium@edge \
      freetype@edge \
      ttf-freefont@edge \
      nss@edge \
      libstdc++@edge \
      sqlite-libs@edge \
      ca-certificates@edge \
 && rm -rf /var/cache/apk/*

WORKDIR /app

ENV CHROMIUM_PATH=/usr/bin/chromium
ENV LAUNCH_HEADLESS=true
ENV PORT=8080

# Download ad/tracker block-list
RUN wget -q https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts \
 && cat hosts >> /etc/hosts \
 && rm hosts

COPY --from=build /app/omnivore .

EXPOSE 8080

CMD ["./omnivore", "server", "content-fetcher"]
