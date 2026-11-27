# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

FROM alpine:3.20 AS runtime

RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S app && adduser -S -G app app

WORKDIR /app

COPY --from=builder /out/api /app/api
COPY --from=builder /out/seed /app/seed
COPY --from=builder /app/migrations /app/migrations

USER app

EXPOSE 8080

ENTRYPOINT ["/app/api"]
