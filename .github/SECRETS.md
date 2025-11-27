# Настройка GitHub Secrets для Kubernetes

Для работы приложения в Kubernetes необходимо настроить следующие секреты в GitHub.

## Добавление секретов

1. Перейдите в Settings → Secrets and variables → Actions
2. Нажмите "New repository secret"
3. Добавьте каждый из следующих секретов:

### Обязательные секреты

| Имя секрета | Описание | Пример |
|------------|----------|--------|
| `REDIS_PASSWORD` | Пароль для Redis | `your_secure_password_123` |
| `SESSION_SECRET` | Секретный ключ для сессий (минимум 32 символа) | `your_very_long_session_secret_key_min_32_chars` |
| `YANDEX_CLIENT_ID` | Client ID для Yandex OAuth | `abc123def456` |
| `YANDEX_CLIENT_SECRET` | Client Secret для Yandex OAuth | `secret_key_from_yandex` |
| `TELEGRAM_TOKEN` | Токен Telegram бота | `123456789:ABCdefGHIjklMNOpqrsTUVwxyz` |
| `SERVICE_USER_ID` | User ID для внутренних сервисов | `service` или ваш user ID |

## Использование с External Secrets Operator

Если вы используете External Secrets Operator, секреты будут автоматически синхронизироваться из GitHub Secrets в Kubernetes Secrets.

См. [k8s/argocd/external-secrets.yaml](../k8s/argocd/external-secrets.yaml) для конфигурации.

## Использование с Helm напрямую

Если вы не используете External Secrets, передайте секреты через Helm:

```bash
helm install gestalt ./k8s/helm/gestalt \
  --namespace gestalt \
  --create-namespace \
  --set secrets.redisPassword="${{ secrets.REDIS_PASSWORD }}" \
  --set secrets.sessionSecret="${{ secrets.SESSION_SECRET }}" \
  --set secrets.yandexClientId="${{ secrets.YANDEX_CLIENT_ID }}" \
  --set secrets.yandexClientSecret="${{ secrets.YANDEX_CLIENT_SECRET }}" \
  --set secrets.telegramToken="${{ secrets.TELEGRAM_TOKEN }}" \
  --set secrets.serviceUserId="${{ secrets.SERVICE_USER_ID }}"
```

## Безопасность

⚠️ **Важно:**
- Никогда не коммитьте секреты в репозиторий
- Используйте разные секреты для разных окружений (dev, staging, production)
- Регулярно ротируйте секреты
- Используйте External Secrets Operator для автоматической синхронизации

