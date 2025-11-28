FROM golang:1.23-alpine AS builder

WORKDIR /app

# Настраиваем кэширование модулей Go через BuildKit cache mount
# Это позволяет кэшировать модули между сборками
ENV GOMODCACHE=/go/pkg/mod
ENV GOCACHE=/go/cache

# Копируем go.mod и go.sum первыми для кэширования зависимостей
COPY backend/go.mod backend/go.sum ./

# Загружаем зависимости с использованием cache mount
# Модули будут кэшироваться между сборками, даже если контейнер пересоздается
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/go/cache \
    go mod download

# Копируем main.go
COPY backend/main.go ./

# Обновляем go.sum если нужно (быстро, т.к. зависимости уже загружены)
# Используем cache mount для ускорения
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/go/cache \
    go mod tidy

# Компилируем приложение с использованием cache mount
# Это ускорит компиляцию при повторных сборках
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/go/cache \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o gestalt ./main.go

# Финальный образ
FROM alpine:latest

WORKDIR /app

# Копируем скомпилированный бинарник из builder
COPY --from=builder /app/gestalt .

# Копируем HTML-файл
COPY frontend/index.html .

EXPOSE 8080

CMD ["./gestalt"]
