FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /caged-mcp ./cmd/mcp-server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates git
COPY --from=builder /caged-mcp /usr/local/bin/caged-mcp
ENTRYPOINT ["/usr/local/bin/caged-mcp"]
CMD ["--mode", "stdio"]
