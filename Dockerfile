# Dockerfile for go-qr-qris
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && go build -o go-qr-qris ./main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/go-qr-qris .
EXPOSE 3000
ENTRYPOINT ["./go-qr-qris"]
