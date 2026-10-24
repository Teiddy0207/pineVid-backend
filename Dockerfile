# syntax=docker/dockerfile:1
# Cache downloaded modules independently from application source.
FROM golang:1.26-alpine3.23 AS modules
WORKDIR /app
COPY go.mod go.sum ./
RUN --mount=type=cache,id=pipevid-backend-modules,target=/go/pkg/mod \
    go mod download

FROM modules AS builder
COPY . .
RUN --mount=type=cache,id=pipevid-backend-modules,target=/go/pkg/mod \
    --mount=type=cache,id=pipevid-backend-build,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -tags migrate -o /bin/app ./cmd/app

FROM scratch
COPY --from=builder /app/config /config
COPY --from=builder /app/migrations /migrations
COPY --from=builder /bin/app /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
CMD ["/app"]
