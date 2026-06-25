FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o dream-checker .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/dream-checker /usr/local/bin/dream-checker
ENTRYPOINT ["dream-checker"]
