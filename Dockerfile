FROM golang:1.27.3-alpine AS builder

WORKDIR /app

COPY go.mod ./

COPY . .
RUN go build -o payment-service .

FROM alpine:3.22

WORKDIR /app

COPY --from=builder /app/payment-service .

CMD ["./payment-service"]