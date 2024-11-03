FROM golang:1.23.2-alpine3.20 as builder
WORKDIR /app
COPY ./ ./
RUN go build -o web ./cmd/web

FROM alpine:3.20 AS prod
WORKDIR /app
COPY --from=builder /app/public /app/public
COPY --from=builder /app/web /app/
ENTRYPOINT ["/app/web"]