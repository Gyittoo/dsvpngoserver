FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod .
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/dsvpn-server ./cmd/server

FROM alpine:3.20

RUN addgroup -S dsvpn && adduser -S dsvpn -G dsvpn
WORKDIR /app

COPY --from=builder /out/dsvpn-server /app/dsvpn-server
COPY --from=builder /app/internal/database/migrations /app/internal/database/migrations

USER dsvpn
EXPOSE 8080

ENTRYPOINT ["/app/dsvpn-server"]
