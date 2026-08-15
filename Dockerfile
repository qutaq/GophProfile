# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOTOOLCHAIN=auto

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
 && go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata curl \
 && adduser -D -u 10001 appuser

WORKDIR /app

COPY --from=builder /out/server /out/worker /out/migrate ./
COPY --from=builder /app/web ./web

USER appuser

EXPOSE 8080 9091

CMD ["./server"]
