FROM golang:1.27-alpine AS builder

WORKDIR /app

# Сначала только файлы зависимостей: если они не менялись, Docker возьмёт этот слой из кэша
COPY go.mod go.sum ./
RUN go mod download

# Потом весь код
COPY . .

# Собираем две программы: API и воркер
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/app
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/worker ./cmd/worker

FROM alpine:latest

WORKDIR /app

COPY --from=builder /out/api /out/worker ./

EXPOSE 8080

CMD ["./api"]
