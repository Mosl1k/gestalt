FROM golang:1.23-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum первыми для кэширования зависимостей
COPY backend/go.mod backend/go.sum ./

# Копируем main.go чтобы go mod tidy мог проанализировать импорты
COPY backend/main.go ./

# Обновляем зависимости и загружаем все необходимые модули
RUN go mod download && go mod tidy

# Компилируем приложение
RUN go build -o gestalt ./main.go

# Финальный образ
FROM alpine:latest

WORKDIR /app

# Копируем скомпилированный бинарник из builder
COPY --from=builder /app/gestalt .

# Копируем HTML-файл
COPY frontend/index.html .

EXPOSE 8080

CMD ["./gestalt"]
