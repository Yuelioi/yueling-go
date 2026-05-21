# ── Build stage ────────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bot ./cmd/bot/

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM alpine:3.20

# ca-certificates: HTTPS; tzdata: 时区支持
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app
COPY --from=builder /build/bot .

# data/ 挂载为外部 volume：图片素材 / 数据库 / 备份 / fortune 资产
VOLUME ["/app/data"]

ENTRYPOINT ["./bot"]
