#!/bin/bash
# Скрипт для настройки cron задачи для бэкапа списка покупок

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKUP_SCRIPT="$SCRIPT_DIR/backup-shopping-list.sh"

# Проверяем, что скрипт бэкапа существует
if [ ! -f "$BACKUP_SCRIPT" ]; then
    echo "Ошибка: скрипт $BACKUP_SCRIPT не найден"
    exit 1
fi

# Делаем скрипт исполняемым
chmod +x "$BACKUP_SCRIPT"

# Проверяем, есть ли уже задача в crontab
CRON_JOB="*/20 * * * * $BACKUP_SCRIPT >> /var/log/shopping-backup.log 2>&1"

if crontab -l 2>/dev/null | grep -q "$BACKUP_SCRIPT"; then
    echo "Задача бэкапа уже настроена в crontab"
    crontab -l | grep "$BACKUP_SCRIPT"
else
    # Добавляем задачу в crontab
    (crontab -l 2>/dev/null; echo "$CRON_JOB") | crontab -
    echo "Задача бэкапа добавлена в crontab:"
    echo "$CRON_JOB"
fi

echo ""
echo "Текущие задачи crontab:"
crontab -l

echo ""
echo "Логи бэкапа будут сохраняться в /var/log/shopping-backup.log"

