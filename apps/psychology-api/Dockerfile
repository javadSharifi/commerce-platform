# apps/psychology-api/Dockerfile
# Multi-stage build for psychology Go backend API

FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/psychology-api ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /bin/psychology-api /app/psychology-api
COPY migrations /app/migrations

ENV PORT=8080
EXPOSE 8080

USER nobody:nobody

ENTRYPOINT ["/app/psychology-api"]
