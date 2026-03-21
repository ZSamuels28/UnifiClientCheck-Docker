# ---- Build stage ----
FROM golang:1.24-alpine AS builder

ARG TARGETOS TARGETARCH

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o unificlientalerts ./cmd/unificlientalerts

# ---- Run stage ----
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

# Run as non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /build/unificlientalerts .

# Create data directory with write permissions and assign to non-root user
RUN mkdir -p /data && chown appuser:appgroup /data && chmod 770 /data

USER appuser

# Persistent data volume for the SQLite database
VOLUME ["/data"]

CMD ["/app/unificlientalerts"]
