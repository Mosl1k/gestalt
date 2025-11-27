# Инструкция по деплою на сервер root@vdska

## Шаги для обновления проекта с авторизацией Yandex OAuth

### 1. Подготовка на сервере

```bash
# Подключитесь к серверу
ssh root@vdska

# Перейдите в директорию проекта
cd ~/gestalt

# Сделайте бэкап текущего состояния
git add .
git commit -m "Backup before OAuth integration" || true
```

### 2. Обновление файлов проекта

Скопируйте обновлённые файлы с локальной машины на сервер:

```bash
# С локальной машины (из директории gestalt-temp)
scp main.go root@vdska:~/gestalt/
scp go.mod root@vdska:~/gestalt/
scp docker-compose.yaml root@vdska:~/gestalt/
scp nginx.conf root@vdska:~/gestalt/
scp Dockerfile.nginx root@vdska:~/gestalt/
```

### 3. Обновление зависимостей Go

```bash
# На сервере
cd ~/gestalt
go mod tidy
go mod download
```

### 4. Синхронизация с GitHub

Если на сервере есть локальные изменения, которые нужно сохранить (например, `backup_redis.sh`, `setup-server.sh`), сделайте их бэкап перед обновлением:

```bash
# На сервере
cd ~/gestalt

# Сохраните локальные файлы, если они нужны
cp backup_redis.sh backup_redis.sh.local
cp setup-server.sh setup-server.sh.local 2>/dev/null || true

# Получите последние изменения из GitHub
git fetch origin
git pull origin main

# Если были конфликты, разрешите их
# Восстановите локальные файлы, если нужно
cp backup_redis.sh.local backup_redis.sh 2>/dev/null || true
```

### 5. Обновление .env файла

Добавьте в `.env` на сервере следующие переменные:

```bash
# Yandex OAuth настройки
YANDEX_CLIENT_ID=ваш_client_id_от_yandex
YANDEX_CLIENT_SECRET=ваш_client_secret_от_yandex
YANDEX_CALLBACK_URL=https://kpalch.ru/auth/yandex/callback

# Секретный ключ для сессий (сгенерируйте новый)
SESSION_SECRET=ваш_случайный_секретный_ключ_минимум_32_символа
```

Для генерации SESSION_SECRET:
```bash
openssl rand -base64 32
```

### 5. Настройка Yandex OAuth

1. Перейдите на https://oauth.yandex.ru/
2. Откройте ваше приложение (или создайте новое)
3. Укажите Callback URI: `https://kpalch.ru/auth/yandex/callback`
4. Выберите необходимые права доступа:
   - Доступ к email адресу
   - Доступ к имени, фамилии и отчеству
5. Скопируйте Client ID и Client Secret в `.env`

### 6. Остановка текущих контейнеров

```bash
# На сервере
cd ~/gestalt
docker-compose down
```

### 7. Остановка внешнего nginx (если запущен)

```bash
# На сервере
systemctl stop nginx
# Или если nginx в Docker
docker stop nginx 2>/dev/null || true
```

### 8. Запуск обновлённого проекта

```bash
# На сервере
cd ~/gestalt
docker-compose build
docker-compose up -d
```

### 9. Получение SSL сертификата (если нужно)

Если сертификат ещё не получен, выполните в контейнере nginx:

```bash
# На сервере
docker exec -it nginx certbot certonly --standalone -d kpalch.ru --non-interactive --agree-tos --email ваш-email@example.com
```

Или используйте веб-интерфейс certbot для первого получения сертификата.

### 10. Проверка работы

```bash
# Проверьте логи
docker-compose logs -f geshtalt
docker-compose logs -f nginx

# Проверьте статус контейнеров
docker-compose ps
```

### 11. Настройка автоматического обновления сертификатов

Добавьте в crontab на сервере:

```bash
# На сервере
crontab -e

# Добавьте строку (обновление сертификата раз в неделю)
0 3 * * 0 docker exec nginx /usr/local/bin/renew-cert.sh
```

### 12. Настройка бэкапов Redis в Яндекс Облако

Если у вас настроено монтирование Яндекс Облако через WebDAV в `/mnt/yandex`:

```bash
# На сервере
# Убедитесь, что скрипт backup_redis.sh существует и исполняемый
chmod +x ~/gestalt/backup_redis.sh

# Добавьте в crontab (бэкап каждый день в 2:00)
crontab -e

# Добавьте строку:
0 2 * * * /bin/bash /root/gestalt/backup_redis.sh >> /mnt/yandex/backup/backup.log 2>&1
```

**Важно**: Скрипт `backup_redis.sh` не должен попадать в git (он уже в `.gitignore`), так как содержит специфичные для сервера пути (`/mnt/yandex/backup`).

## Откат изменений (если что-то пошло не так)

```bash
# На сервере
cd ~/gestalt
docker-compose down
git checkout HEAD -- main.go go.mod docker-compose.yaml nginx.conf
systemctl start nginx  # если использовали внешний nginx
docker-compose up -d
```

## Важные замечания

1. **Порты**: Убедитесь, что порты 80 и 443 свободны на сервере
2. **Сертификаты**: Если у вас уже есть сертификаты в `/etc/letsencrypt`, они будут использованы
3. **Бэкап**: Всегда делайте бэкап перед обновлением
4. **Тестирование**: Протестируйте авторизацию перед полным деплоем

