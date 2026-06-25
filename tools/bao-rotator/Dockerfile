FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o bao-rotator ./cmd

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/bao-rotator /usr/local/bin/bao-rotator
ENTRYPOINT ["bao-rotator"]
