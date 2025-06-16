FROM golang:1.22-alpine AS builder

WORKDIR /app

# Копируем go.mod и go.sum первыми для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем остальной код
COPY . .

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