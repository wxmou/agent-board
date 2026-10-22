FROM node:22-alpine AS web

WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.24-alpine AS build

RUN apk add --no-cache ca-certificates
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/webui/dist ./webui/dist
ARG VERSION=1.0.0
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/agentboard ./cmd/agentboard

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S agentboard \
    && adduser -S agentboard -G agentboard \
    && mkdir -p /data /app \
    && chown -R agentboard:agentboard /data /app

WORKDIR /app
COPY --from=build /out/agentboard /app/agentboard

USER agentboard
ENV APP_ENV=production \
    AGENT_BOARD_ADDR=:8080 \
    AGENT_BOARD_DB=/data/agent-board.db \
    TZ=Asia/Shanghai

VOLUME ["/data"]
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/api/health >/dev/null || exit 1

ENTRYPOINT ["/app/agentboard"]
