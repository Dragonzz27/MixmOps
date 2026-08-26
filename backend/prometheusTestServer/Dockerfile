FROM golang:alpine AS builder

WORKDIR /app
# The Docker build context is backend/, which contains the Go module and
# the Prometheus test server source.

COPY . .
WORKDIR /app/prometheusTestServer
RUN CGO_ENABLED=0 GOOS=linux go build -o testserver main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/prometheusTestServer/testserver .
EXPOSE 2112
CMD ["./testserver"]
