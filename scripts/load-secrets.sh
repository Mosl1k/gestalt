#!/bin/bash

# Скрипт для загрузки секретов из GitHub Secrets или .env файла
# Приоритет: GitHub Secrets > .env файл

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$PROJECT_ROOT/.env"
GITHUB_REPO="${GITHUB_REPOSITORY:-Mosl1k/gestalt}"

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

# Проверяем наличие GitHub CLI
if command -v gh &> /dev/null; then
    # Проверяем авторизацию в GitHub
    if gh auth status &> /dev/null; then
        info "GitHub CLI найден и авторизован"
        USE_GITHUB_SECRETS=true
    else
        warn "GitHub CLI найден, но не авторизован. Используем .env файл"
        USE_GITHUB_SECRETS=false
    fi
else
    warn "GitHub CLI не найден. Используем .env файл"
    USE_GITHUB_SECRETS=false
fi

# Список необходимых секретов
REQUIRED_SECRETS=(
    "REDIS_PASSWORD"
    "SESSION_SECRET"
    "YANDEX_CLIENT_ID"
    "YANDEX_CLIENT_SECRET"
    "TELEGRAM_TOKEN"
    "SERVICE_USER_ID"
)

OPTIONAL_SECRETS=(
    "DOMAIN"
    "YANDEX_CALLBACK_URL"
)

# Функция для получения секрета из GitHub
get_github_secret() {
    local secret_name=$1
    if [ "$USE_GITHUB_SECRETS" = true ]; then
        # GitHub CLI может получить секреты только через Actions
        # Для получения секретов репозитория используем другой подход
        # Пока что возвращаем пустую строку, так как gh secret get работает только для Actions secrets
        # В будущем можно использовать GitHub API с токеном
        echo ""
    else
        echo ""
    fi
}

# Функция для получения значения из .env файла
get_env_value() {
    local key=$1
    local file_to_read="${2:-$EXISTING_ENV_FILE}"
    if [ -n "$file_to_read" ] && [ -f "$file_to_read" ]; then
        grep "^${key}=" "$file_to_read" | cut -d '=' -f2- | sed 's/^"//;s/"$//' || echo ""
    else
        echo ""
    fi
}

# Создаем или обновляем .env файл
info "Загрузка секретов..."

# Сохраняем существующий .env для чтения (если есть)
EXISTING_ENV_FILE="$ENV_FILE"
if [ -f "$ENV_FILE" ]; then
    # Проверяем, есть ли в существующем .env все необходимые переменные
    ALL_PRESENT=true
    for secret in "${REQUIRED_SECRETS[@]}"; do
        if ! grep -q "^${secret}=" "$ENV_FILE"; then
            ALL_PRESENT=false
            break
        fi
    done
    
    if [ "$ALL_PRESENT" = true ]; then
        info "Все необходимые переменные уже присутствуют в .env файле"
        info "Используем существующий .env файл (GitHub Secrets будут использованы только если доступны)"
        # Не перезаписываем .env, если все переменные уже есть
        # Но все равно пытаемся обновить из GitHub Secrets
        USE_EXISTING=true
    else
        # Создаем резервную копию
        cp "$ENV_FILE" "${ENV_FILE}.backup.$(date +%Y%m%d_%H%M%S)"
        info "Создана резервная копия .env файла"
        USE_EXISTING=false
    fi
else
    EXISTING_ENV_FILE=""
    USE_EXISTING=false
fi

# Создаем новый .env файл только если не используем существующий
if [ "$USE_EXISTING" != true ]; then
    > "$ENV_FILE"
fi

# Загружаем обязательные секреты
MISSING_SECRETS=()
for secret in "${REQUIRED_SECRETS[@]}"; do
    value=""
    
    # Пытаемся получить из GitHub Secrets
    if [ "$USE_GITHUB_SECRETS" = true ]; then
        value=$(get_github_secret "$secret")
        if [ -n "$value" ]; then
            debug "Секрет $secret загружен из GitHub Secrets"
        fi
    fi
    
    # Если не получили из GitHub, берем из существующего .env файла
    if [ -z "$value" ] && [ -n "$EXISTING_ENV_FILE" ]; then
        value=$(get_env_value "$secret" "$EXISTING_ENV_FILE")
        if [ -n "$value" ]; then
            debug "Секрет $secret загружен из существующего .env файла"
        fi
    fi
    
    # Если все еще нет значения
    if [ -z "$value" ]; then
        # Если используем существующий .env и переменная там есть, берем оттуда
        if [ "$USE_EXISTING" = true ] && grep -q "^${secret}=" "$EXISTING_ENV_FILE"; then
            value=$(get_env_value "$secret" "$EXISTING_ENV_FILE")
            if [ -n "$value" ]; then
                debug "Секрет $secret сохранен из существующего .env файла"
            fi
        fi
        
        if [ -z "$value" ]; then
            warn "Секрет $secret не найден ни в GitHub Secrets, ни в .env файле"
            MISSING_SECRETS+=("$secret")
            continue
        fi
    fi
    
    # Записываем в .env файл только если не используем существующий или значение изменилось
    if [ "$USE_EXISTING" != true ] || ! grep -q "^${secret}=${value}$" "$ENV_FILE" 2>/dev/null; then
        # Удаляем старую строку если есть
        if [ "$USE_EXISTING" = true ]; then
            sed -i "/^${secret}=/d" "$ENV_FILE"
        fi
        echo "${secret}=${value}" >> "$ENV_FILE"
    fi
done

# Загружаем опциональные секреты
for secret in "${OPTIONAL_SECRETS[@]}"; do
    value=""
    
    # Пытаемся получить из GitHub Secrets
    if [ "$USE_GITHUB_SECRETS" = true ]; then
        value=$(get_github_secret "$secret")
    fi
    
    # Если не получили из GitHub, берем из существующего .env
    if [ -z "$value" ] && [ -n "$EXISTING_ENV_FILE" ]; then
        value=$(get_env_value "$secret" "$EXISTING_ENV_FILE")
    fi
    
    # Если есть значение, записываем
    if [ -n "$value" ]; then
        echo "${secret}=${value}" >> "$ENV_FILE"
    fi
done

# Добавляем стандартные переменные, если их нет
if ! grep -q "^REDIS_ADDR=" "$ENV_FILE"; then
    echo "REDIS_ADDR=redis:6379" >> "$ENV_FILE"
fi

if ! grep -q "^API_URL=" "$ENV_FILE"; then
    echo "API_URL=http://geshtalt:8080/internal/api" >> "$ENV_FILE"
fi

# Если есть домен, добавляем YANDEX_CALLBACK_URL если его нет
if grep -q "^DOMAIN=" "$ENV_FILE" && ! grep -q "^YANDEX_CALLBACK_URL=" "$ENV_FILE"; then
    DOMAIN=$(grep "^DOMAIN=" "$ENV_FILE" | cut -d '=' -f2-)
    echo "YANDEX_CALLBACK_URL=https://${DOMAIN}/auth/yandex/callback" >> "$ENV_FILE"
fi

# Проверяем наличие всех обязательных секретов
if [ ${#MISSING_SECRETS[@]} -gt 0 ]; then
    error "Отсутствуют следующие обязательные секреты:"
    for secret in "${MISSING_SECRETS[@]}"; do
        echo "  - $secret"
    done
    echo ""
    echo "Добавьте их в GitHub Secrets или в .env файл"
    exit 1
fi

info "Секреты успешно загружены в .env файл"
info "Использовано: $([ "$USE_GITHUB_SECRETS" = true ] && echo "GitHub Secrets" || echo ".env файл")"

