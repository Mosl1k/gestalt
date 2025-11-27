#!/bin/bash
# Скрипт для деплоя обновлённых файлов на сервер

SERVER="root@vdska"
REMOTE_DIR="~/gestalt"
LOCAL_DIR="/Users/koyash/golang/gestalt-temp"

# Цвета
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${YELLOW}=== Деплой обновлённых файлов на сервер ===${NC}"
echo "Сервер: $SERVER"
echo "Удалённая директория: $REMOTE_DIR"
echo ""

# Проверяем SSH подключение
echo -e "${YELLOW}Проверка SSH подключения...${NC}"
ssh -o ConnectTimeout=5 -o BatchMode=yes "$SERVER" exit 2>/dev/null
if [ $? -ne 0 ]; then
    echo -e "${RED}✗ Не удалось подключиться к серверу $SERVER${NC}"
    exit 1
fi
echo -e "${GREEN}✓ SSH подключение установлено${NC}"

# Файлы для копирования
FILES=(
    "main.go"
    "go.mod"
    "docker-compose.yaml"
    "nginx.conf"
    "Dockerfile.nginx"
)

echo ""
echo -e "${YELLOW}Копирование файлов на сервер...${NC}"
for file in "${FILES[@]}"; do
    if [ -f "$LOCAL_DIR/$file" ]; then
        echo "  → $file"
        scp "$LOCAL_DIR/$file" "$SERVER:$REMOTE_DIR/$file"
        if [ $? -eq 0 ]; then
            echo -e "    ${GREEN}✓${NC}"
        else
            echo -e "    ${RED}✗ Ошибка${NC}"
        fi
    else
        echo -e "  ${RED}✗ Файл $file не найден${NC}"
    fi
done

echo ""
echo -e "${GREEN}=== Готово! ===${NC}"
echo ""
echo "Следующие шаги на сервере:"
echo "1. cd ~/gestalt"
echo "2. go mod tidy"
echo "3. Обновите .env файл (добавьте YANDEX_CLIENT_ID, YANDEX_CLIENT_SECRET, SESSION_SECRET)"
echo "4. docker-compose down"
echo "5. docker-compose build"
echo "6. docker-compose up -d"
echo ""
echo "Подробная инструкция в файле DEPLOY.md"

