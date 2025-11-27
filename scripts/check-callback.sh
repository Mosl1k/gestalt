#!/bin/bash
# Скрипт для проверки настроек callback URL

SERVER="root@vdska"

echo "=== Проверка настроек callback URL ==="
echo ""

echo "1. Callback URL в .env файле:"
ssh "$SERVER" "cd ~/gestalt && grep YANDEX_CALLBACK_URL .env 2>/dev/null || echo 'YANDEX_CALLBACK_URL не найден в .env'"

echo ""
echo "2. Callback URL по умолчанию в коде:"
echo "   https://kpalch.ru/auth/yandex/callback"

echo ""
echo "3. Проверка переменных окружения в контейнере:"
ssh "$SERVER" "docker exec geshtalt env | grep YANDEX || echo 'Переменные YANDEX не найдены'"

echo ""
echo "=== Что нужно сделать ==="
echo "1. Убедитесь, что в .env на сервере указано:"
echo "   YANDEX_CALLBACK_URL=https://kpalch.ru/auth/yandex/callback"
echo ""
echo "2. В настройках Yandex OAuth (https://oauth.yandex.ru/) добавьте:"
echo "   Callback URI: https://kpalch.ru/auth/yandex/callback"
echo ""
echo "3. Перезапустите контейнер после обновления .env:"
echo "   docker-compose restart geshtalt"

