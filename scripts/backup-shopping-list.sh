#!/bin/bash
# Скрипт для бэкапа списка покупок в Яндекс.Облако
# Запускается каждые 20 минут через cron

set -e

# Путь к файлу бэкапа
BACKUP_DIR="/mnt/yandex/gestalt"
BACKUP_FILE="$BACKUP_DIR/shopping.txt"
TEMP_FILE="/tmp/shopping_backup_$$.txt"

# Проверяем, что Яндекс.Облако смонтировано
if [ ! -d "$BACKUP_DIR" ]; then
    echo "Ошибка: директория $BACKUP_DIR не существует. Проверьте монтирование Яндекс.Облако."
    exit 1
fi

# Создаем директорию, если её нет
mkdir -p "$BACKUP_DIR"

# Получаем переменные окружения из .env или из системы
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

if [ -f "$PROJECT_DIR/.env" ]; then
    source "$PROJECT_DIR/.env"
elif [ -f "/root/gestalt/.env" ]; then
    source /root/gestalt/.env
elif [ -f ".env" ]; then
    source .env
fi

# Определяем способ подключения к Redis (Docker или прямой)
REDIS_CONTAINER="redis"
USE_DOCKER=false

# Проверяем, запущен ли Redis в Docker
if docker ps --format '{{.Names}}' | grep -q "^${REDIS_CONTAINER}$"; then
    USE_DOCKER=true
    echo "Используется Docker контейнер: $REDIS_CONTAINER"
fi

# Функция для получения данных из Redis
get_redis_data() {
    local key=$1
    if [ "$USE_DOCKER" = true ]; then
        # Через Docker контейнер
        if [ -n "$REDIS_PASSWORD" ]; then
            docker exec "$REDIS_CONTAINER" redis-cli -a "$REDIS_PASSWORD" GET "$key" 2>/dev/null || echo ""
        else
            docker exec "$REDIS_CONTAINER" redis-cli GET "$key" 2>/dev/null || echo ""
        fi
    else
        # Прямое подключение
        REDIS_HOST="${REDIS_HOST:-localhost}"
        REDIS_PORT="${REDIS_PORT:-6379}"
        if [ -n "$REDIS_PASSWORD" ]; then
            redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" GET "$key" 2>/dev/null || echo ""
        else
            redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" GET "$key" 2>/dev/null || echo ""
        fi
    fi
}

# Функция для получения всех ключей списков
get_all_list_keys() {
    if [ "$USE_DOCKER" = true ]; then
        # Через Docker контейнер
        if [ -n "$REDIS_PASSWORD" ]; then
            docker exec "$REDIS_CONTAINER" redis-cli -a "$REDIS_PASSWORD" KEYS "shoppingList:*" 2>/dev/null || echo ""
        else
            docker exec "$REDIS_CONTAINER" redis-cli KEYS "shoppingList:*" 2>/dev/null || echo ""
        fi
    else
        # Прямое подключение
        REDIS_HOST="${REDIS_HOST:-localhost}"
        REDIS_PORT="${REDIS_PORT:-6379}"
        if [ -n "$REDIS_PASSWORD" ]; then
            redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASSWORD" KEYS "shoppingList:*" 2>/dev/null || echo ""
        else
            redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" KEYS "shoppingList:*" 2>/dev/null || echo ""
        fi
    fi
}

# Проверяем доступность Redis
if [ "$USE_DOCKER" = true ]; then
    if ! docker exec "$REDIS_CONTAINER" redis-cli ${REDIS_PASSWORD:+-a "$REDIS_PASSWORD"} PING >/dev/null 2>&1; then
        echo "Ошибка: не удалось подключиться к Redis в контейнере $REDIS_CONTAINER"
        exit 1
    fi
else
    REDIS_HOST="${REDIS_HOST:-localhost}"
    REDIS_PORT="${REDIS_PORT:-6379}"
    if ! redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" ${REDIS_PASSWORD:+-a "$REDIS_PASSWORD"} PING >/dev/null 2>&1; then
        echo "Ошибка: не удалось подключиться к Redis ($REDIS_HOST:$REDIS_PORT)"
        exit 1
    fi
fi

# Начинаем формировать файл бэкапа
{
    echo "=== Бэкап списка покупок ==="
    echo "Дата: $(date '+%Y-%m-%d %H:%M:%S')"
    echo ""
    
    # Получаем все ключи списков
    LIST_KEYS=$(get_all_list_keys)
    
    if [ -z "$LIST_KEYS" ]; then
        echo "Списки покупок пусты"
        exit 0
    fi
    
    # Обрабатываем каждый список
    for key in $LIST_KEYS; do
        # Извлекаем userID и category из ключа (формат: shoppingList:userID:category)
        IFS=':' read -r prefix user_id category <<< "$key"
        
        # Получаем данные списка
        list_data=$(get_redis_data "$key")
        
        if [ -z "$list_data" ] || [ "$list_data" = "null" ]; then
            continue
        fi
        
        # Парсим JSON и форматируем
        echo "--- Пользователь: $user_id | Категория: $category ---"
        
        # Используем Python для парсинга JSON (если доступен)
        if command -v python3 >/dev/null 2>&1; then
            echo "$list_data" | python3 -c "
import json
import sys
try:
    data = json.load(sys.stdin)
    if isinstance(data, list):
        for item in data:
            status = '✓' if item.get('bought', False) else '☐'
            priority = '🔥' if item.get('priority', 2) == 3 else ('🟡' if item.get('priority', 2) == 2 else '🟢')
            print(f\"  {status} {priority} {item.get('name', '')}\")
    else:
        print(f\"  {data}\")
except:
    print(f\"  {sys.stdin.read()}\")
" 2>/dev/null || echo "  $list_data"
        else
            # Если Python недоступен, просто выводим JSON
            echo "  $list_data"
        fi
        
        echo ""
    done
    
    echo "=== Конец бэкапа ==="
} > "$TEMP_FILE"

# Атомически перемещаем файл на место
mv "$TEMP_FILE" "$BACKUP_FILE"

echo "Бэкап успешно сохранен в $BACKUP_FILE"
exit 0

