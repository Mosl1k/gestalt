#!/bin/bash

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

debug() {
    echo -e "${BLUE}[DEBUG]${NC} $1"
}

# Проверка, что мы в git репозитории
if [ ! -d ".git" ]; then
    error "Это не git репозиторий. Используйте обычный deploy.sh"
    exit 1
fi

# Определяем базовую ветку для сравнения (можно передать как аргумент)
BASE_BRANCH=${1:-origin/main}
FORCE_REBUILD=${2:-false}
COMPOSE_FILE="docker/docker-compose.yaml"
COMPOSE_DIR="docker"

# Если передан флаг --force или --all, пересобираем все
if [ "$1" = "--force" ] || [ "$1" = "--all" ] || [ "$FORCE_REBUILD" = "true" ]; then
    FORCE_REBUILD=true
    BASE_BRANCH=origin/main
fi

# Переходим в директорию проекта
cd "$(dirname "$0")/.."
PROJECT_ROOT=$(pwd)

# Загружаем секреты из GitHub Secrets или .env
if [ -f "$PROJECT_ROOT/scripts/load-secrets.sh" ]; then
    info "Загрузка секретов..."
    "$PROJECT_ROOT/scripts/load-secrets.sh" || warn "Не удалось загрузить секреты, используем существующий .env"
fi

info "Умный деплой Gestalt"
info "Сравнение с: $BASE_BRANCH"
echo ""

# Получаем список измененных файлов
info "Анализ изменений..."
CHANGED_FILES=$(git diff --name-only $BASE_BRANCH HEAD 2>/dev/null || git diff --name-only HEAD~1 HEAD 2>/dev/null || echo "")

if [ -z "$CHANGED_FILES" ] || [ "$(echo "$CHANGED_FILES" | wc -l)" -eq 0 ]; then
    warn "Не найдено изменений между $BASE_BRANCH и HEAD."
    warn "Используется последний коммит для анализа."
    CHANGED_FILES=$(git diff-tree --no-commit-id --name-only -r HEAD 2>/dev/null || echo "")
fi

if [ -z "$CHANGED_FILES" ] || [ "$(echo "$CHANGED_FILES" | grep -v '^$' | wc -l)" -eq 0 ]; then
    warn "Не удалось определить изменения."
    if [ "$FORCE_REBUILD" != "true" ]; then
        info "Нет изменений для деплоя. Используйте --force для принудительной пересборки."
        exit 0
    else
        CHANGED_FILES="ALL"
    fi
fi

debug "Измененные файлы:"
echo "$CHANGED_FILES" | while read file; do
    if [ ! -z "$file" ]; then
        debug "  - $file"
    fi
done
echo ""

# Определяем какие сервисы нужно пересобрать
declare -A SERVICES_MAP

# Анализируем измененные файлы
REBUILD_ALL=false

if [ "$FORCE_REBUILD" = "true" ]; then
    REBUILD_ALL=true
    info "Принудительная пересборка всех сервисов"
elif [ "$CHANGED_FILES" = "ALL" ]; then
    # Если не удалось определить изменения, пересобираем все
    REBUILD_ALL=true
    info "Пересобираю все сервисы"
else
    while IFS= read -r file; do
        if [ -z "$file" ]; then
            continue
        fi
        
        debug "Анализ файла: $file"
        
        # Бекенд (Go)
        if [[ "$file" == backend/* ]] || [[ "$file" == "go.mod" ]] || [[ "$file" == "go.sum" ]]; then
            SERVICES_MAP["geshtalt"]=1
            debug "  → geshtalt (изменен: $file)"
        fi
        
        # Фронтенд
        if [[ "$file" == frontend/* ]]; then
            SERVICES_MAP["geshtalt"]=1
            debug "  → geshtalt (изменен: $file)"
        fi
        
        # Сервис Алисы
        if [[ "$file" == services/alice/* ]]; then
            SERVICES_MAP["alice"]=1
            debug "  → alice (изменен: $file)"
        fi
        
        # Телеграм бот
        if [[ "$file" == services/telegram-bot/* ]]; then
            SERVICES_MAP["telegram-bot"]=1
            debug "  → telegram-bot (изменен: $file)"
        fi
        
        # Docker файлы
        if [[ "$file" == docker/Dockerfile ]]; then
            SERVICES_MAP["geshtalt"]=1
            debug "  → geshtalt (изменен Dockerfile)"
        fi
        if [[ "$file" == docker/Dockerfile.python ]]; then
            SERVICES_MAP["alice"]=1
            debug "  → alice (изменен Dockerfile.python)"
        fi
        if [[ "$file" == docker/Dockerfile.bot ]]; then
            SERVICES_MAP["telegram-bot"]=1
            debug "  → telegram-bot (изменен Dockerfile.bot)"
        fi
        if [[ "$file" == docker/Dockerfile.nginx ]]; then
            SERVICES_MAP["nginx"]=1
            debug "  → nginx (изменен Dockerfile.nginx)"
        fi
        
        # docker-compose.yaml - пересобираем все
        if [[ "$file" == docker/docker-compose.yaml ]] || [[ "$file" == docker-compose.yaml ]]; then
            REBUILD_ALL=true
            warn "docker-compose.yaml изменен - пересобираю все сервисы"
            break
        fi
        
        # Nginx конфигурация - только перезапуск
        if [[ "$file" == infra/nginx/* ]]; then
            SERVICES_MAP["nginx"]=1
            debug "  → nginx (изменена конфигурация)"
        fi
        
        # Redis конфигурация - только перезапуск (не пересборка)
        if [[ "$file" == infra/redis/* ]]; then
            SERVICES_MAP["redis"]=1
            debug "  → redis (изменена конфигурация)"
        fi
    done <<< "$CHANGED_FILES"
fi

# Формируем список сервисов
if [ "$REBUILD_ALL" = true ]; then
    UNIQUE_SERVICES=("geshtalt" "alice" "telegram-bot" "nginx")
else
    UNIQUE_SERVICES=($(printf '%s\n' "${!SERVICES_MAP[@]}" | sort -u))
fi

if [ ${#UNIQUE_SERVICES[@]} -eq 0 ]; then
    info "Нет сервисов для пересборки. Изменения не затрагивают сервисы."
    exit 0
fi

info "Сервисы для пересборки/перезапуска:"
for service in "${UNIQUE_SERVICES[@]}"; do
    echo "  - $service"
done
echo ""

# Переходим в директорию docker
cd "$COMPOSE_DIR"

# Определяем, нужен ли полный down для некоторых сервисов
# (например, при изменении docker-compose.yaml или критических изменений)
NEED_FULL_DOWN=false
if [ "$REBUILD_ALL" = true ] || [ ${#UNIQUE_SERVICES[@]} -gt 2 ]; then
    NEED_FULL_DOWN=true
    warn "Много изменений - используем docker-compose down для затронутых сервисов"
fi

# Останавливаем только нужные сервисы
if [ "$NEED_FULL_DOWN" = true ]; then
    info "Остановка всех затронутых сервисов через docker-compose down..."
    # Останавливаем только нужные сервисы
    docker-compose stop "${UNIQUE_SERVICES[@]}" 2>/dev/null || true
    # Удаляем контейнеры для пересборки
    for service in "${UNIQUE_SERVICES[@]}"; do
        if [ "$service" != "redis" ]; then
            docker-compose rm -f "$service" 2>/dev/null || true
        fi
    done
else
    info "Остановка сервисов..."
    for service in "${UNIQUE_SERVICES[@]}"; do
        if [ "$service" = "redis" ]; then
            # Redis не пересобираем, только перезапускаем
            info "Перезапуск redis..."
            docker-compose restart redis || true
        else
            info "Остановка $service..."
            docker-compose stop "$service" 2>/dev/null || true
            # Удаляем контейнер для пересоздания с правильным именем
            docker-compose rm -f "$service" 2>/dev/null || true
        fi
    done
fi

# Пересобираем только нужные сервисы
info "Пересборка сервисов..."
for service in "${UNIQUE_SERVICES[@]}"; do
    if [ "$service" = "redis" ]; then
        continue  # Redis не пересобираем
    fi
    info "Сборка $service..."
    docker-compose build "$service"
done

# Запускаем сервисы
info "Запуск сервисов..."
for service in "${UNIQUE_SERVICES[@]}"; do
    if [ "$service" = "redis" ]; then
        continue  # Redis уже перезапущен
    fi
    info "Запуск $service..."
    docker-compose up -d "$service"
done

# Показываем статус
echo ""
info "Статус сервисов:"
docker-compose ps

echo ""
info "========================================="
info "Деплой завершен!"
info "========================================="
info "Пересобрано сервисов: ${#UNIQUE_SERVICES[@]}"
info "Логи: cd $COMPOSE_DIR && docker-compose logs -f"

