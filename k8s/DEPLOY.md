# Пошаговое развертывание в Kubernetes

## Подготовка

### 1. Настройка GitHub Secrets

Убедитесь, что все секреты добавлены в GitHub:
- Settings → Secrets and variables → Actions

Необходимые секреты:
- `REDIS_PASSWORD`
- `SESSION_SECRET`
- `YANDEX_CLIENT_ID`
- `YANDEX_CLIENT_SECRET`
- `TELEGRAM_TOKEN`
- `SERVICE_USER_ID`

### 2. Подготовка Kubernetes кластера

```bash
# Проверьте доступ к кластеру
kubectl cluster-info

# Создайте namespace
kubectl create namespace gestalt
```

### 3. Установка ArgoCD (опционально, но рекомендуется)

```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Получите пароль администратора
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d && echo

# Пробросьте порт для доступа к UI
kubectl port-forward svc/argocd-server -n argocd 8080:443
```

## Развертывание

### Вариант 1: Через ArgoCD (рекомендуется)

```bash
# Примените ArgoCD Application
kubectl apply -f k8s/argocd/application.yaml

# ArgoCD автоматически синхронизирует приложение
# Проверьте статус в UI: http://localhost:8080
```

### Вариант 2: Через Helm напрямую

```bash
# Установите Helm чарт
helm install gestalt ./k8s/helm/gestalt \
  --namespace gestalt \
  --create-namespace \
  --set domain.autoDetect=true \
  --set domain.certbotEmail="your-email@example.com" \
  --set secrets.redisPassword="$REDIS_PASSWORD" \
  --set secrets.sessionSecret="$SESSION_SECRET" \
  --set secrets.yandexClientId="$YANDEX_CLIENT_ID" \
  --set secrets.yandexClientSecret="$YANDEX_CLIENT_SECRET" \
  --set secrets.telegramToken="$TELEGRAM_TOKEN" \
  --set secrets.serviceUserId="$SERVICE_USER_ID"
```

### Вариант 3: С указанием домена вручную

Если автоматическое определение домена не работает:

```bash
helm install gestalt ./k8s/helm/gestalt \
  --namespace gestalt \
  --create-namespace \
  --set domain.autoDetect=false \
  --set domain.name="your-domain.com" \
  --set domain.certbotEmail="your-email@example.com" \
  --set secrets.redisPassword="$REDIS_PASSWORD" \
  --set secrets.sessionSecret="$SESSION_SECRET" \
  --set secrets.yandexClientId="$YANDEX_CLIENT_ID" \
  --set secrets.yandexClientSecret="$YANDEX_CLIENT_SECRET" \
  --set secrets.telegramToken="$TELEGRAM_TOKEN" \
  --set secrets.serviceUserId="$SERVICE_USER_ID"
```

## Проверка развертывания

```bash
# Проверьте статус подов
kubectl get pods -n gestalt

# Проверьте сервисы
kubectl get svc -n gestalt

# Проверьте логи
kubectl logs -l app.kubernetes.io/component=geshtalt -n gestalt --tail=50

# Проверьте логи nginx (для проверки определения домена)
kubectl logs -l app.kubernetes.io/component=nginx -n gestalt --tail=50
```

## Автоматическое определение домена

Система автоматически определяет домен через:
1. Получение внешнего IP сервера (через api.ipify.org)
2. Reverse DNS lookup (dig -x)
3. Если домен не найден, используется IP адрес

Домен автоматически используется для:
- Nginx конфигурации (server_name)
- SSL сертификатов (certbot)
- Yandex OAuth callback URL

## Обновление домена в Yandex OAuth

После развертывания:
1. Определите домен: `kubectl get configmap gestalt-domain -n gestalt -o jsonpath='{.data.DOMAIN}'`
2. Обновите Callback URI в Yandex OAuth: `https://<ваш-домен>/auth/yandex/callback`
3. Обновите секрет `YANDEX_CALLBACK_URL` в GitHub Secrets (если используете External Secrets)

## Troubleshooting

### Домен не определяется

```bash
# Проверьте логи init-контейнера
kubectl logs -l app.kubernetes.io/component=nginx -n gestalt -c detect-domain-and-cert

# Проверьте ConfigMap с доменом
kubectl get configmap gestalt-domain -n gestalt -o yaml
```

### SSL сертификат не получен

```bash
# Проверьте логи certbot
kubectl logs -l app.kubernetes.io/component=nginx -n gestalt -c detect-domain-and-cert | grep certbot

# Убедитесь, что DNS записи настроены правильно
dig your-domain.com
```

### Проблемы с секретами

```bash
# Проверьте наличие секретов
kubectl get secrets -n gestalt

# Проверьте содержимое секрета (base64)
kubectl get secret gestalt-secrets -n gestalt -o yaml
```

