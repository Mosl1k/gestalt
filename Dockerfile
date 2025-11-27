FROM golang:1.23-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum первыми для кэширования зависимостей
COPY go.mod go.sum ./

# Копируем main.go чтобы go mod tidy мог проанализировать импорты
COPY main.go ./

# Обновляем зависимости и загружаем все необходимые модули
RUN go mod download && go mod tidy

# Копируем остальной код (исключая go.mod и go.sum, которые уже обновлены)
COPY index.html ./

# Компилируем приложение
RUN go build -o gestalt ./main.go

# Финальный образ
FROM alpine:latest

WORKDIR /app

# Копируем скомпилированный бинарник из builder
COPY --from=builder /app/gestalt .

# Копируем HTML-файл, если он нужен
COPY index.html .

EXPOSE 8080

CMD ["./gestalt"]
