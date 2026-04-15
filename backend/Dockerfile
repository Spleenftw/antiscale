FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY . .
# Now that main.go is copied, go mod tidy can accurately map and fetch dependencies
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o controller main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/controller .

EXPOSE 8080
CMD ["./controller"]
