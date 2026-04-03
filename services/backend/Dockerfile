FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -ldflags="-s -w" -o /backend ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /backend /usr/local/bin/backend

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/backend"]
