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
# Используем -x для более быстрой загрузки и проверяем все зависимости
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/go/cache \
    go mod download -x all

# Копируем main.go
COPY backend/main.go ./

# Обновляем go.sum только если нужно (быстро, т.к. зависимости уже загружены)
# Используем cache mount для ускорения
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/go/cache \
    go mod tidy || true

# Компилируем приложение с использованием cache mount и параллельной компиляции
# Это ускорит компиляцию при повторных сборках
# -ldflags="-w -s" убирает отладочную информацию и уменьшает размер
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/go/cache \
    CGO_ENABLED=0 GOOS=linux \
    go build -ldflags="-w -s" -trimpath -o gestalt ./main.go

# Финальный образ
FROM alpine:latest

WORKDIR /app

# Копируем скомпилированный бинарник из builder
COPY --from=builder /app/gestalt .

# Копируем HTML-файл
COPY frontend/index.html .

EXPOSE 8080

CMD ["./gestalt"]
