# ── WebUI stage ───────────────────────────────────────────────────────────────
FROM node:24-alpine AS webui

WORKDIR /webui
COPY webui/package.json webui/pnpm-lock.yaml webui/pnpm-workspace.yaml ./
RUN corepack enable && corepack prepare pnpm@11.1.2 --activate && pnpm install --frozen-lockfile
COPY webui/ ./
RUN pnpm build

# ── Build stage ────────────────────────────────────────────────────────────────
FROM golang:1.27.2-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=webui /webui/dist ./webui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bot ./cmd/bot/

FROM node:24-alpine AS codex
ARG CODEX_VERSION=0.151.0
RUN npm install --global --prefix /opt/codex @openai/codex@${CODEX_VERSION}

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM alpine:3.23

# ca-certificates: HTTPS; tzdata: 时区支持
RUN apk add --no-cache ca-certificates tzdata nodejs libstdc++

WORKDIR /app
COPY --from=builder /build/bot .
COPY --from=codex /opt/codex /opt/codex
ENV PATH="/opt/codex/bin:${PATH}"
COPY --from=webui /webui/dist ./webui/dist

EXPOSE 9077 9078 9080

# data/ 挂载为外部 volume：图片素材 / 备份 / fortune 资产（结构化数据在 PostgreSQL）
VOLUME ["/app/data"]

ENTRYPOINT ["./bot"]
