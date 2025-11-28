FROM golang:1.23-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum первыми для кэширования зависимостей
COPY backend/go.mod backend/go.sum ./

# Загружаем зависимости (кэшируется если go.mod/go.sum не изменились)
RUN go mod download

# Копируем main.go
COPY backend/main.go ./

# Компилируем приложение (используем кэш зависимостей из предыдущего шага)
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o gestalt ./main.go

# Финальный образ
FROM alpine:latest

WORKDIR /app

# Копируем скомпилированный бинарник из builder
COPY --from=builder /app/gestalt .

# Копируем HTML-файл
COPY frontend/index.html .

EXPOSE 8080

CMD ["./gestalt"]
