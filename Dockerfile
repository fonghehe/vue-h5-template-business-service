# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Build stage: compile the server. CGO is required because the sqlite driver
# (mattn/go-sqlite3) is linked in; it is used only by the demo/seed path and
# unit tests, never by production traffic (PostgreSQL is the runtime database).
# ---------------------------------------------------------------------------
FROM golang:1.25-alpine AS build

# VERSION is stamped into the binary so the health endpoint reports a real tag.
ARG VERSION=dev

RUN apk add --no-cache build-base git

WORKDIR /src

# Cache module downloads before copying the source tree.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/server \
    ./cmd/server

# ---------------------------------------------------------------------------
# Runtime stage: minimal image running as a non-root user.
# ---------------------------------------------------------------------------
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S app \
    && adduser -S -G app -u 10001 app

WORKDIR /app
COPY --from=build /out/server /app/server

USER app
EXPOSE 8002

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8002/health >/dev/null 2>&1 || exit 1

CMD ["/app/server"]
