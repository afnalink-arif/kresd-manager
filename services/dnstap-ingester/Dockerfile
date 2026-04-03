FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -ldflags="-s -w" -o /dnstap-ingester .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /dnstap-ingester /usr/local/bin/dnstap-ingester

ENTRYPOINT ["/usr/local/bin/dnstap-ingester"]
