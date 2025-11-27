#!/bin/bash
# Скрипт для автоматического определения доменного имени по IP адресу сервера

set -e

# Получаем внешний IP сервера
get_server_ip() {
    # Пробуем разные способы получения IP
    if command -v curl >/dev/null 2>&1; then
        SERVER_IP=$(curl -s https://api.ipify.org || curl -s https://ifconfig.me || curl -s https://icanhazip.com)
    elif command -v wget >/dev/null 2>&1; then
        SERVER_IP=$(wget -qO- https://api.ipify.org || wget -qO- https://ifconfig.me)
    else
        # Если нет curl/wget, пробуем через hostname
        SERVER_IP=$(hostname -I | awk '{print $1}')
    fi
    
    if [ -z "$SERVER_IP" ]; then
        echo "Ошибка: не удалось определить IP адрес сервера" >&2
        exit 1
    fi
    
    echo "$SERVER_IP"
}

# Определяем домен по IP через reverse DNS
detect_domain() {
    local ip=$1
    
    # Пробуем reverse DNS lookup
    if command -v dig >/dev/null 2>&1; then
        DOMAIN=$(dig +short -x "$ip" 2>/dev/null | sed 's/\.$//' || echo "")
    elif command -v host >/dev/null 2>&1; then
        DOMAIN=$(host "$ip" 2>/dev/null | awk '{print $5}' | sed 's/\.$//' || echo "")
    elif command -v nslookup >/dev/null 2>&1; then
        DOMAIN=$(nslookup "$ip" 2>/dev/null | grep "name" | awk '{print $4}' | sed 's/\.$//' || echo "")
    fi
    
    # Если reverse DNS не дал результат, пробуем через HTTP заголовки
    if [ -z "$DOMAIN" ] || [ "$DOMAIN" = "NXDOMAIN" ]; then
        # Пробуем получить домен из HTTP заголовков (если есть веб-сервер)
        if command -v curl >/dev/null 2>&1; then
            DOMAIN=$(curl -sI "http://$ip" 2>/dev/null | grep -i "server:" | awk '{print $2}' || echo "")
        fi
    fi
    
    # Если всё ещё нет домена, используем IP как fallback
    if [ -z "$DOMAIN" ] || [ "$DOMAIN" = "NXDOMAIN" ]; then
        echo "Предупреждение: не удалось определить домен по IP $ip, будет использован IP адрес" >&2
        DOMAIN="$ip"
    fi
    
    echo "$DOMAIN"
}

# Основная функция
main() {
    local server_ip
    local domain
    
    server_ip=$(get_server_ip)
    domain=$(detect_domain "$server_ip")
    
    # Выводим результат
    echo "$domain"
}

# Если скрипт вызван напрямую
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    main "$@"
fi

