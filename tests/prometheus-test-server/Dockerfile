FROM golang:alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o testserver main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/testserver .
EXPOSE 2112
CMD ["./testserver"]
