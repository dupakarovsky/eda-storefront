# customers/Dockerfile
FROM golang:1.26 AS builder

WORKDIR /workspace

COPY foundation/ ./foundation
COPY customers/ ./customers

WORKDIR /workspace/customers

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/customers ./cmd/service

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /bin/customers ./customers

EXPOSE 8080

CMD ["./customers"]
