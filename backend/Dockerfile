FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod ./
# Download dependencies - since we use `go get` locally we will allow it to fail gracefully if go.sum is missing or we just init module
RUN go mod tidy || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o controller main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/controller .

EXPOSE 8080
CMD ["./controller"]
