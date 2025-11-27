#!/bin/bash
# Скрипт для синхронизации сервера с GitHub
# Использование: ./sync-server.sh

SERVER="root@vdska"
REMOTE_DIR="~/gestalt"

echo "=== Синхронизация сервера с GitHub ==="
echo ""

# Проверяем SSH подключение
echo "Проверка SSH подключения..."
ssh -o ConnectTimeout=5 -o BatchMode=yes "$SERVER" exit 2>/dev/null
if [ $? -ne 0 ]; then
    echo "✗ Не удалось подключиться к серверу $SERVER"
    echo "Проверьте SSH ключи и доступность сервера"
    exit 1
fi
echo "✓ SSH подключение установлено"
echo ""

# Выполняем команды на сервере
echo "Выполнение синхронизации на сервере..."
ssh "$SERVER" << 'ENDSSH'
cd ~/gestalt

echo "Текущий статус git:"
git status --short

echo ""
echo "Сохранение локальных файлов (backup_redis.sh, setup-server.sh)..."
[ -f backup_redis.sh ] && cp backup_redis.sh backup_redis.sh.local || echo "backup_redis.sh не найден"
[ -f setup-server.sh ] && cp setup-server.sh setup-server.sh.local || echo "setup-server.sh не найден"

echo ""
echo "Получение изменений из GitHub..."
git fetch origin

echo ""
echo "Обновление из main..."
git pull origin main

echo ""
echo "Восстановление локальных файлов (если нужно)..."
[ -f backup_redis.sh.local ] && cp backup_redis.sh.local backup_redis.sh && echo "✓ backup_redis.sh восстановлен" || echo "backup_redis.sh.local не найден"
[ -f setup-server.sh.local ] && cp setup-server.sh.local setup-server.sh && echo "✓ setup-server.sh восстановлен" || echo "setup-server.sh.local не найден"

echo ""
echo "Финальный статус:"
git status --short

echo ""
echo "=== Синхронизация завершена ==="
echo ""
echo "Следующие шаги:"
echo "1. Проверьте изменения: git diff"
echo "2. Если нужно, пересоберите контейнеры: docker-compose build"
echo "3. Перезапустите сервисы: docker-compose up -d"
ENDSSH

if [ $? -eq 0 ]; then
    echo ""
    echo "✓ Синхронизация успешно завершена!"
else
    echo ""
    echo "✗ Ошибка при синхронизации"
    exit 1
fi

