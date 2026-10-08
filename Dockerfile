FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
COPY . .


RUN go mod download
RUN CGO_ENABLED=O GOOS=linux go build -o main ./cmd/app/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/main .

EXPOSE 8080

CMD ["./main"]