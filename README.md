# Gestalt - Система управления списками покупок

Веб-приложение для управления списками покупок с поддержкой нескольких интерфейсов:
- Веб-интерфейс (OAuth2 через Yandex)
- Телеграм-бот
- Навык Яндекс Алисы

## Структура проекта

```
gestalt/
├── backend/              # Go бекенд
│   ├── main.go          # Основной код приложения
│   ├── go.mod           # Go зависимости
│   └── go.sum
├── frontend/            # Фронтенд
│   └── index.html       # HTML интерфейс
├── services/            # Микросервисы
│   ├── alice/          # Сервис для Яндекс Алисы
│   │   ├── alice.py
│   │   └── requirements.txt
│   └── telegram-bot/   # Телеграм бот
│       ├── telegram_bot.py
│       └── requirements_bot.txt
├── docker/              # Docker конфигурация
│   ├── docker-compose.yaml
│   ├── Dockerfile       # Бекенд
│   ├── Dockerfile.bot   # Телеграм бот
│   ├── Dockerfile.python # Алиса
│   └── Dockerfile.nginx # Nginx
├── infra/               # Инфраструктура
│   ├── nginx/
│   │   └── nginx.conf
│   └── redis/
│       ├── redis.conf
│       └── backup_redis.sh
└── scripts/             # Скрипты
    ├── deploy.sh        # Развертывание на новом сервере
    ├── check-callback.sh
    ├── deploy-to-server.sh
    └── sync-server.sh
```

## Архитектура

### API Endpoints

1. **Публичные endpoints** (без авторизации):
   - `GET /` - Главная страница
   - `GET /auth/yandex` - Авторизация через Yandex OAuth
   - `GET /auth/yandex/callback` - Callback от Yandex
   - `GET /logout` - Выход

2. **Защищенные endpoints** (OAuth2 авторизация):
   - `GET /list?category=...` - Получить список
   - `POST /add` - Добавить элемент
   - `PUT /buy/{name}` - Отметить как купленное
   - `DELETE /delete/{name}?category=...` - Удалить элемент
   - `PUT /edit/{name}?oldCategory=...` - Редактировать элемент
   - `POST /reorder` - Изменить порядок элементов
   - API для друзей: `/api/user`, `/api/friends/*`, `/api/users/*`, `/api/shared-lists`, `/api/share-list`

3. **Внутренние API endpoints** (только из Docker сети, без авторизации):
   - `GET /internal/api/list?category=...` - Для сервисов
   - `POST /internal/api/add` - Для сервисов
   - `PUT /internal/api/buy/{name}` - Для сервисов
   - `DELETE /internal/api/delete/{name}?category=...` - Для сервисов
   - `PUT /internal/api/edit/{name}?oldCategory=...` - Для сервисов

### Сервисы

- **geshtalt** (Go бекенд) - основной API сервер на порту 8080
- **redis** - база данных
- **alice** (Python Flask) - сервис для Яндекс Алисы на порту 2112
- **telegram-bot** (Python) - телеграм бот
- **nginx** - reverse proxy на портах 80/443

## Быстрый старт

### Локальная разработка (Docker Compose)

1. Клонируйте репозиторий
2. Создайте `.env` файл на основе `.env.example`:
```bash
cp .env.example .env
# Отредактируйте .env и заполните необходимые значения
```
3. Запустите через Docker Compose:

```bash
cd docker
docker-compose up --build
```

### Production развертывание (Kubernetes)

Проект настроен для развертывания в Kubernetes с использованием Helm и ArgoCD.

**Предварительные требования:**
- Kubernetes кластер (1.24+)
- Helm 3.0+
- ArgoCD (опционально, для GitOps)

**Быстрая установка:**

1. Настройте GitHub Secrets (см. [k8s/README.md](k8s/README.md))
2. Установите ArgoCD:
```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```
3. Разверните приложение:
```bash
kubectl apply -f k8s/argocd/application.yaml
```

**Подробная документация:** [k8s/README.md](k8s/README.md)

**Миграция с Docker Compose:** [MIGRATION.md](MIGRATION.md)

## Переменные окружения

### Для локальной разработки (.env)

```bash
# Redis
REDIS_ADDR=redis:6379
REDIS_PASSWORD=your_redis_password

# Yandex OAuth
YANDEX_CLIENT_ID=your_client_id
YANDEX_CLIENT_SECRET=your_client_secret
YANDEX_CALLBACK_URL=https://your-domain.com/auth/yandex/callback

# Session
SESSION_SECRET=your_session_secret_min_32_chars

# Telegram Bot
TELEGRAM_TOKEN=your_telegram_token
API_URL=http://geshtalt:8080/internal/api
SERVICE_USER_ID=your_service_user_id  # User ID для сервисов (опционально)
```

### Для Kubernetes (GitHub Secrets)

В Kubernetes переменные окружения управляются через:
- **ConfigMaps** - для несекретных данных
- **Secrets** - для секретных данных (синхронизируются из GitHub Secrets)

Добавьте следующие секреты в GitHub Repository Settings → Secrets and variables → Actions:
- `REDIS_PASSWORD`
- `SESSION_SECRET`
- `YANDEX_CLIENT_ID`
- `YANDEX_CLIENT_SECRET`
- `TELEGRAM_TOKEN`
- `SERVICE_USER_ID`

## Бэкапы

Бэкапы Redis автоматически создаются в `/mnt/yandex/backup/` (если настроено монтирование Яндекс.Облако).

Для ручного бэкапа:
```bash
./infra/redis/backup_redis.sh
```

Для автоматических бэкапов добавьте в crontab:
```bash
0 2 * * * /bin/bash /root/gestalt/infra/redis/backup_redis.sh >> /mnt/yandex/backup/backup.log 2>&1
```

## Разработка

### Добавление нового сервиса

1. Создайте папку в `services/`
2. Создайте Dockerfile в `docker/`
3. Добавьте сервис в `docker/docker-compose.yaml`
4. Используйте внутренний API: `http://geshtalt:8080/internal/api/...`

### Изменение бекенда

1. Измените код в `backend/main.go`
2. Пересоберите образ: `cd docker && docker-compose build geshtalt`
3. Перезапустите: `docker-compose restart geshtalt`

### Изменение фронтенда

1. Измените `frontend/index.html`
2. Пересоберите образ: `cd docker && docker-compose build geshtalt`
3. Перезапустите: `docker-compose restart geshtalt`

## Безопасность

- Веб-интерфейс использует OAuth2 через Yandex
- Внутренние API доступны только из Docker сети (проверка по IP адресу)
- Сервисы общаются между собой через внутреннюю Docker сеть
- SSL/TLS настроен через nginx с Let's Encrypt сертификатами

## Устранение неполадок

### Телеграм бот не работает

1. Проверьте `TELEGRAM_TOKEN` в `.env`
2. Проверьте `API_URL=http://geshtalt:8080/internal/api`
3. Проверьте логи: `docker-compose logs telegram-bot`

### Алиса не работает

1. Проверьте логи: `docker-compose logs alice`
2. Убедитесь, что nginx правильно проксирует запросы на `/alice/`

### Redis не восстанавливается

1. Проверьте наличие бэкапов в `/mnt/yandex/backup/`
2. Проверьте права доступа к файлам бэкапа
3. Проверьте логи Redis: `docker-compose logs redis`

## Лицензия

MIT
