#!/bin/sh
# Init-скрипт для автоматического определения домена и получения SSL сертификата

set -e

# Функция определения домена по IP
detect_domain() {
    # Получаем внешний IP
    SERVER_IP=$(curl -s https://api.ipify.org || curl -s https://ifconfig.me || echo "")
    
    if [ -z "$SERVER_IP" ]; then
        echo "Ошибка: не удалось определить IP адрес сервера"
        exit 1
    fi
    
    echo "Определен IP сервера: $SERVER_IP"
    
    # Определяем домен через reverse DNS
    if command -v dig >/dev/null 2>&1; then
        DOMAIN=$(dig +short -x "$SERVER_IP" 2>/dev/null | sed 's/\.$//' || echo "")
    elif command -v host >/dev/null 2>&1; then
        DOMAIN=$(host "$SERVER_IP" 2>/dev/null | awk '{print $5}' | sed 's/\.$//' || echo "")
    fi
    
    # Если reverse DNS не дал результат, используем IP
    if [ -z "$DOMAIN" ] || [ "$DOMAIN" = "NXDOMAIN" ]; then
        echo "Предупреждение: не удалось определить домен по IP, будет использован IP: $SERVER_IP"
        DOMAIN="$SERVER_IP"
    fi
    
    echo "Определен домен: $DOMAIN"
    echo "$DOMAIN"
}

# Получаем домен
DOMAIN=$(detect_domain)

# Сохраняем домен в файл для использования в основном контейнере
echo "$DOMAIN" > /shared/domain.txt

# Если домен - это IP, пропускаем получение сертификата
if echo "$DOMAIN" | grep -qE '^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$'; then
    echo "Домен является IP адресом, пропускаем получение SSL сертификата"
    exit 0
fi

# Получаем email для Let's Encrypt из переменной окружения или используем дефолтный
CERTBOT_EMAIL="${CERTBOT_EMAIL:-admin@${DOMAIN}}"

# Проверяем, есть ли уже сертификат
if [ -f "/etc/letsencrypt/live/${DOMAIN}/fullchain.pem" ]; then
    echo "Сертификат для $DOMAIN уже существует"
    exit 0
fi

# Получаем сертификат через certbot
echo "Получение SSL сертификата для $DOMAIN..."

# Используем standalone режим для получения сертификата
certbot certonly \
    --standalone \
    -d "$DOMAIN" \
    --non-interactive \
    --agree-tos \
    --email "$CERTBOT_EMAIL" \
    --preferred-challenges http \
    || {
        echo "Ошибка при получении сертификата. Возможно, домен не указывает на этот сервер."
        echo "Убедитесь, что DNS записи настроены правильно."
        exit 1
    }

echo "Сертификат успешно получен для $DOMAIN"

