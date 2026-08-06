# ============================================================
#  DSVPN Backend — Multi-stage Production Build
#  Stage 1: Build Go binary (golang:1-alpine — matches go.mod)
#  Stage 2: Run in minimal image (alpine:3.20)
# ============================================================

# ---- Build Stage ----
FROM golang:1-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o /out/dsvpn-server ./cmd/server

# ---- Production Stage ----
FROM alpine:3.20

LABEL maintainer="DSVPN Team"
LABEL description="DSVPN Backend API Server"

RUN apk add --no-cache ca-certificates wget tzdata \
    && addgroup -S dsvpn && adduser -S dsvpn -G dsvpn

WORKDIR /app

COPY --from=builder /out/dsvpn-server /app/dsvpn-server
COPY --from=builder /app/internal/database/migrations /app/internal/database/migrations

RUN chown -R dsvpn:dsvpn /app

USER dsvpn

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO /dev/null http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/dsvpn-server"]
