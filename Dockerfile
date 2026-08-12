# ── WebUI stage ───────────────────────────────────────────────────────────────
FROM node:22-alpine AS webui

WORKDIR /webui
COPY webui/package.json webui/pnpm-lock.yaml webui/pnpm-workspace.yaml ./
RUN corepack enable && corepack prepare pnpm@11.1.2 --activate && pnpm install --frozen-lockfile
COPY webui/ ./
RUN pnpm build

# ── Build stage ────────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=webui /webui/dist ./webui/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bot ./cmd/bot/ \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o migrate-sqlite ./cmd/migrate-sqlite/

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM alpine:3.20

# ca-certificates: HTTPS; tzdata: 时区支持
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /build/bot .
COPY --from=builder /build/migrate-sqlite .
COPY --from=webui /webui/dist ./webui/dist

EXPOSE 9077 9078 9080

# data/ 挂载为外部 volume：图片素材 / 备份 / fortune 资产（结构化数据在 PostgreSQL）
VOLUME ["/app/data"]

ENTRYPOINT ["./bot"]
