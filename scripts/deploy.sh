#!/bin/bash

set -e  # Остановка при ошибке

echo "========================================="
echo "Скрипт развертывания Gestalt на новом сервере"
echo "========================================="

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Функция для вывода сообщений
info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Проверка, что скрипт запущен от root
if [ "$EUID" -ne 0 ]; then 
    error "Пожалуйста, запустите скрипт от root: sudo $0"
    exit 1
fi

# Переменные
PROJECT_DIR="/root/gestalt"
BACKUP_DIR="/mnt/yandex/backup"
YANDEX_MOUNT="/mnt/yandex"
ENV_FILE="$PROJECT_DIR/.env"

# Шаг 1: Проверка и установка зависимостей
info "Проверка зависимостей..."

if ! command -v docker &> /dev/null; then
    warn "Docker не установлен. Устанавливаю..."
    curl -fsSL https://get.docker.com -o get-docker.sh
    sh get-docker.sh
    rm get-docker.sh
    systemctl enable docker
    systemctl start docker
    info "Docker установлен"
else
    info "Docker уже установлен"
fi

if ! command -v docker-compose &> /dev/null; then
    warn "Docker Compose не установлен. Устанавливаю..."
    curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
    chmod +x /usr/local/bin/docker-compose
    info "Docker Compose установлен"
else
    info "Docker Compose уже установлен"
fi

# Шаг 2: Создание директорий
info "Создание директорий проекта..."
mkdir -p "$PROJECT_DIR"
mkdir -p "$YANDEX_MOUNT"
mkdir -p "$BACKUP_DIR"

# Шаг 3: Монтирование Яндекс.Облако (если не смонтировано)
if ! mountpoint -q "$YANDEX_MOUNT"; then
    warn "Яндекс.Облако не смонтировано. Нужно настроить монтирование."
    echo "Для монтирования Яндекс.Облако через WebDAV используйте:"
    echo "  apt-get install davfs2 -y"
    echo "  echo 'https://webdav.yandex.ru /mnt/yandex davfs user,noauto 0 0' >> /etc/fstab"
    echo "  mount /mnt/yandex"
    echo ""
    read -p "Продолжить без монтирования? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        error "Прервано пользователем"
        exit 1
    fi
else
    info "Яндекс.Облако уже смонтировано"
fi

# Шаг 4: Клонирование репозитория (если не существует)
if [ ! -d "$PROJECT_DIR/.git" ]; then
    info "Клонирование репозитория..."
    read -p "Введите URL репозитория Git (или нажмите Enter для пропуска): " GIT_REPO
    if [ ! -z "$GIT_REPO" ]; then
        git clone "$GIT_REPO" "$PROJECT_DIR"
    else
        warn "Репозиторий не клонирован. Убедитесь, что код находится в $PROJECT_DIR"
    fi
else
    info "Репозиторий уже существует"
fi

# Шаг 5: Загрузка секретов из GitHub Secrets или .env файла
info "Загрузка секретов..."

# Сначала пытаемся загрузить из GitHub Secrets или существующего .env
if [ -f "$PROJECT_ROOT/scripts/load-secrets.sh" ]; then
    if "$PROJECT_ROOT/scripts/load-secrets.sh"; then
        info "Секреты загружены из GitHub Secrets или существующего .env"
    else
        warn "Не удалось загрузить секреты через load-secrets.sh"
        # Пробуем загрузить из Яндекс.Облако как fallback
        if [ -f "$YANDEX_MOUNT/.env" ]; then
            info "Найден .env файл в Яндекс.Облако. Копирую..."
            cp "$YANDEX_MOUNT/.env" "$ENV_FILE"
            info ".env файл скопирован"
        elif [ -f "$BACKUP_DIR/.env" ]; then
            info "Найден .env файл в директории бэкапов. Копирую..."
            cp "$BACKUP_DIR/.env" "$ENV_FILE"
            info ".env файл скопирован"
        else
            error ".env файл не найден ни в GitHub Secrets, ни в Яндекс.Облако"
            error "Пожалуйста, настройте GitHub Secrets или создайте .env файл"
            exit 1
        fi
    fi
else
    # Если скрипт load-secrets.sh не найден, используем старый способ
    warn "Скрипт load-secrets.sh не найден, используем старый способ загрузки .env"
    if [ -f "$YANDEX_MOUNT/.env" ]; then
        info "Найден .env файл в Яндекс.Облако. Копирую..."
        cp "$YANDEX_MOUNT/.env" "$ENV_FILE"
        info ".env файл скопирован"
    elif [ -f "$BACKUP_DIR/.env" ]; then
        info "Найден .env файл в директории бэкапов. Копирую..."
        cp "$BACKUP_DIR/.env" "$ENV_FILE"
        info ".env файл скопирован"
    else
        error ".env файл не найден"
        error "Пожалуйста, создайте .env файл или настройте GitHub Secrets"
        exit 1
    fi
fi

# Шаг 6: Получение последнего бэкапа Redis
info "Поиск последнего бэкапа Redis..."
if [ -d "$BACKUP_DIR" ] && [ "$(ls -A $BACKUP_DIR 2>/dev/null)" ]; then
    # Находим последний dump.rdb файл
    LATEST_DUMP=$(ls -t "$BACKUP_DIR"/dump_*.rdb 2>/dev/null | head -1)
    LATEST_AOF_DIR=$(ls -td "$BACKUP_DIR"/appendonlydir_* 2>/dev/null | head -1)
    
    if [ ! -z "$LATEST_DUMP" ]; then
        info "Найден последний бэкап: $(basename $LATEST_DUMP)"
        BACKUP_TIMESTAMP=$(basename "$LATEST_DUMP" | sed 's/dump_\(.*\)\.rdb/\1/')
        info "Метка времени бэкапа: $BACKUP_TIMESTAMP"
    else
        warn "Бэкапы Redis не найдены. Будет создан новый пустой Redis"
    fi
else
    warn "Директория бэкапов пуста или не существует"
fi

# Шаг 7: Переход в директорию проекта
cd "$PROJECT_DIR"

# Шаг 8: Обновление кода (если это git репозиторий)
if [ -d ".git" ]; then
    info "Обновление кода из репозитория..."
    git pull origin main || warn "Не удалось обновить код из репозитория"
fi

# Шаг 9: Создание docker-compose.yaml в корне (симлинк или копия)
if [ ! -f "docker-compose.yaml" ]; then
    if [ -f "docker/docker-compose.yaml" ]; then
        info "Создаю симлинк на docker-compose.yaml..."
        ln -s docker/docker-compose.yaml docker-compose.yaml
    else
        error "docker-compose.yaml не найден!"
        exit 1
    fi
fi

# Шаг 10: Восстановление Redis из бэкапа (если есть)
if [ ! -z "$LATEST_DUMP" ]; then
    info "Восстановление Redis из бэкапа..."
    
    # Создаем volume для Redis, если его нет
    if ! docker volume inspect gestalt_redis_data >/dev/null 2>&1; then
        info "Создание volume для Redis..."
        docker volume create gestalt_redis_data
    fi
    
    # Временно запускаем Redis для восстановления
    info "Восстановление данных Redis..."
    docker run --rm \
        -v gestalt_redis_data:/data \
        -v "$BACKUP_DIR":/backup \
        redis:latest \
        sh -c "cp /backup/dump_$BACKUP_TIMESTAMP.rdb /data/dump.rdb && \
               if [ -d /backup/appendonlydir_$BACKUP_TIMESTAMP ]; then \
                   mkdir -p /data/appendonlydir && \
                   cp -r /backup/appendonlydir_$BACKUP_TIMESTAMP/* /data/appendonlydir/; \
               fi"
    
    info "Данные Redis восстановлены"
else
    info "Бэкап не найден, будет создан новый Redis"
fi

# Шаг 11: Сборка и запуск контейнеров
info "Сборка Docker образов..."
cd docker
docker-compose build

info "Запуск контейнеров..."
docker-compose up -d

# Шаг 12: Ожидание запуска сервисов
info "Ожидание запуска сервисов..."
sleep 10

# Шаг 13: Проверка статуса
info "Проверка статуса контейнеров..."
docker-compose ps

# Шаг 14: Показ логов
info "Последние логи (Ctrl+C для выхода)..."
docker-compose logs --tail=50

echo ""
info "========================================="
info "Развертывание завершено!"
info "========================================="
info "Проект находится в: $PROJECT_DIR"
info "Логи: cd $PROJECT_DIR/docker && docker-compose logs -f"
info "Остановка: cd $PROJECT_DIR/docker && docker-compose down"
info "Перезапуск: cd $PROJECT_DIR/docker && docker-compose restart"
echo ""
warn "Не забудьте:"
warn "1. Проверить .env файл: $ENV_FILE"
warn "2. Настроить SSL сертификаты (если нужно)"
warn "3. Настроить автоматические бэкапы в crontab"
warn "4. Настроить обновление SSL сертификатов"

