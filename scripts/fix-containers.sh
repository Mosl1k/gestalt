#!/bin/bash

set -e

# Цвета для вывода
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

info "Исправление имен контейнеров"

# Переходим в директорию docker
cd docker

# Останавливаем и удаляем старые контейнеры с неправильными именами
info "Остановка старых контейнеров..."
docker-compose stop 2>/dev/null || true

# Удаляем старые контейнеры
info "Удаление старых контейнеров..."
docker-compose rm -f 2>/dev/null || true

# Также удаляем контейнеры по старым именам вручную
OLD_CONTAINERS=("gestalt-alice" "gestalt-nginx" "gestalt-geshtalt" "gestalt-telegram-bot-1" "gestalt-telegram-bot")

for container in "${OLD_CONTAINERS[@]}"; do
    if docker ps -a --format '{{.Names}}' | grep -q "^${container}$"; then
        warn "Удаление старого контейнера: $container"
        docker stop "$container" 2>/dev/null || true
        docker rm -f "$container" 2>/dev/null || true
    fi
done

# Пересобираем и запускаем с правильными именами
info "Пересборка и запуск контейнеров с правильными именами..."
docker-compose build
docker-compose up -d

info "Проверка контейнеров..."
docker-compose ps

echo ""
info "Готово! Контейнеры пересозданы с правильными именами."

