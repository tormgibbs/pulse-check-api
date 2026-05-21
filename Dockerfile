FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
  -ldflags="-w -s" \
  -o pulse-check-api \
  ./cmd/api

FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /app/pulse-check-api .

COPY --from=builder /app/internal/db/migrations ./internal/db/migrations

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/app/pulse-check-api"]
