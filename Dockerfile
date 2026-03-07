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
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/unificlientalerts .

# Create data directory for SQLite database with appropriate permissions
RUN mkdir -p /data && chmod 777 /data

# Persistent data volume for the SQLite database
VOLUME ["/data"]

CMD ["/app/unificlientalerts"]
