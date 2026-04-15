FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod tidy || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o antiscale-client main.go

FROM alpine:latest
# Install wireguard-tools for the host bindings (for future MVP expansion)
RUN apk add --no-cache curl wireguard-tools iptables

WORKDIR /app
COPY --from=builder /app/antiscale-client .

# For WireGuard execution via exec.Command, we need to add the appropriate capabilities.
# This should be supplied via docker run --cap-add=NET_ADMIN or in docker-compose.
CMD ["./antiscale-client"]
