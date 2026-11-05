# Stage 1: Build Go binaries
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/migrate ./cmd/migrate

# Stage 2: Runtime
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata
# Headless Chromium for the government filing agent's browser channel
# (only the worker uses it; chromedp finds chromium on PATH, or set CHROME_PATH).
RUN apk add --no-cache chromium nss freetype harfbuzz ttf-freefont
RUN adduser -D -u 1000 appuser
# Private (non-public) files: prepared returns, portal evidence, signed PDFs
RUN mkdir -p /data/private && chown appuser /data/private
ENV GOV_FILES_DIR=/data/private/gov DOC_FILES_DIR=/data/private/documents

COPY --from=builder /bin/api /bin/api
COPY --from=builder /bin/worker /bin/worker
COPY --from=builder /bin/migrate /bin/migrate
COPY db/migrations /migrations

USER appuser
EXPOSE 8080
CMD ["/bin/api"]
