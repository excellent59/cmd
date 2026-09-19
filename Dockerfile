# --- ЭТАП 1: Сборка (Builder) ---
FROM golang:1.26.5-alpine AS builder

# Устанавливаем необходимые зависимости для сборки
RUN apk add --no-cache git ca-certificates tzdata

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем бинарный файл (статическая линковка, без CGO)
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o pro-lot-bot ./cmd/bot

# --- ЭТАП 2: Финальный образ (Runner) ---
FROM alpine:latest

# Устанавливаем часовые пояса и корневые сертификаты (нужны для HTTPS и Telegram API)
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Копируем скомпилированный бинарник из этапа builder
COPY --from=builder /app/pro-lot-bot .
COPY --from=builder /app/migrations ./migrations

# Делаем файл исполняемым
RUN chmod +x pro-lot-bot

# Запускаем приложение
CMD ["./pro-lot-bot"]

EXPOSE 8080