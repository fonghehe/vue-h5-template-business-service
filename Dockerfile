# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Build stage: PostgreSQL runtime needs no C compiler. SQLite tests run outside
# this image with CGO enabled.
# ---------------------------------------------------------------------------
FROM golang:1.25-alpine AS build

# VERSION is stamped into the binary for the startup log.
ARG VERSION=dev

RUN apk add --no-cache git

WORKDIR /src

# Cache module downloads before copying the source tree.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/server \
    ./cmd/server
RUN CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate

# ---------------------------------------------------------------------------
# Runtime stage: minimal image running as a non-root user.
# ---------------------------------------------------------------------------
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S app \
    && adduser -S -G app -u 10001 app

WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /out/migrate /app/migrate

USER app
EXPOSE 8002

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:${PORT:-8002}/health >/dev/null 2>&1 || exit 1

CMD ["/app/server"]
