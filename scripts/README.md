# Скрипты развертывания

## smart-deploy.sh

Умный скрипт деплоя, который автоматически определяет какие сервисы нужно пересобрать на основе измененных файлов.

### Использование

```bash
# Автоматическое определение изменений (сравнение с origin/main)
./scripts/smart-deploy.sh

# Сравнение с конкретной веткой/коммитом
./scripts/smart-deploy.sh origin/develop

# Принудительная пересборка всех сервисов
./scripts/smart-deploy.sh --force
# или
./scripts/smart-deploy.sh --all
```

### Логика определения сервисов

Скрипт анализирует измененные файлы и определяет какие сервисы нужно пересобрать:

- `backend/*`, `go.mod`, `go.sum`, `frontend/*` → **geshtalt**
- `services/alice/*` → **alice**
- `services/telegram-bot/*` → **telegram-bot**
- `docker/Dockerfile*` → соответствующий сервис
- `docker/docker-compose.yaml` → **все сервисы**
- `infra/nginx/*` → **nginx** (только перезапуск)
- `infra/redis/*` → **redis** (только перезапуск)

### Особенности

1. **Умная пересборка**: Пересобирает только измененные сервисы
2. **Оптимизация**: Redis и nginx (при изменении только конфига) не пересобираются, только перезапускаются
3. **Полный down**: При большом количестве изменений или изменении docker-compose.yaml использует `docker-compose down` для затронутых сервисов
4. **Безопасность**: Не останавливает сервисы, которые не были изменены

### Примеры

```bash
# Изменили только телеграм-бота
git commit -m "Fix telegram bot navigation"
./scripts/smart-deploy.sh
# → Пересоберет только telegram-bot

# Изменили бекенд и фронтенд
git commit -m "Update backend and frontend"
./scripts/smart-deploy.sh
# → Пересоберет только geshtalt

# Изменили docker-compose.yaml
git commit -m "Update docker-compose"
./scripts/smart-deploy.sh
# → Пересоберет все сервисы
```

## deploy.sh

Полный скрипт развертывания на новом сервере. Используется только при первом развертывании или полной переустановке.

```bash
sudo ./scripts/deploy.sh
```

## backup_redis.sh

Скрипт для создания бэкапа Redis. Сохраняет данные в `/mnt/yandex/backup/`.

```bash
./infra/redis/backup_redis.sh
```

