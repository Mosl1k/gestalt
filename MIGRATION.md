# Миграция на Kubernetes

Этот документ описывает процесс миграции с Docker Compose на Kubernetes.

## Преимущества Kubernetes

- **Масштабируемость**: Легко масштабировать отдельные сервисы
- **Отказоустойчивость**: Автоматический перезапуск подов при сбоях
- **GitOps**: Автоматическое развертывание через ArgoCD
- **Секреты**: Безопасное управление секретами через Kubernetes Secrets
- **Мониторинг**: Интеграция с Prometheus, Grafana и другими инструментами
- **Обновления**: Rolling updates без простоя

## Шаги миграции

### 1. Подготовка Kubernetes кластера

```bash
# Установите kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Установите Helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

### 2. Установка ArgoCD

```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Получите пароль администратора
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
```

### 3. Настройка GitHub Secrets

Добавьте следующие секреты в GitHub Repository Settings → Secrets and variables → Actions:

- `REDIS_PASSWORD`
- `SESSION_SECRET`
- `YANDEX_CLIENT_ID`
- `YANDEX_CLIENT_SECRET`
- `TELEGRAM_TOKEN`
- `SERVICE_USER_ID`

### 4. Миграция данных Redis

#### Экспорт данных из Docker Compose

```bash
# На старом сервере
docker exec redis redis-cli --rdb /tmp/dump.rdb
docker cp redis:/tmp/dump.rdb ./dump.rdb
```

#### Импорт в Kubernetes

```bash
# После развертывания Redis в k8s
kubectl cp dump.rdb gestalt-redis-0:/data/dump.rdb -n gestalt
kubectl exec -it gestalt-redis-0 -n gestalt -- redis-cli --rdb /data/dump.rdb
```

### 5. Развертывание через ArgoCD

```bash
kubectl apply -f k8s/argocd/application.yaml
```

### 6. Проверка развертывания

```bash
# Проверка статуса подов
kubectl get pods -n gestalt

# Проверка сервисов
kubectl get svc -n gestalt

# Логи
kubectl logs -l app.kubernetes.io/name=gestalt -n gestalt
```

### 7. Обновление DNS

Обновите DNS записи, чтобы указывать на LoadBalancer IP nginx сервиса:

```bash
kubectl get svc gestalt-nginx -n gestalt
```

## Откат на Docker Compose

Если что-то пошло не так, можно вернуться:

```bash
# Остановите Kubernetes развертывание
kubectl delete -f k8s/argocd/application.yaml

# Запустите Docker Compose
cd docker
docker-compose up -d
```

## Часто задаваемые вопросы

### Как обновить приложение?

Просто запушьте изменения в `main` ветку. ArgoCD автоматически синхронизирует изменения.

### Как посмотреть логи?

```bash
kubectl logs -l app.kubernetes.io/component=geshtalt -n gestalt --tail=100 -f
```

### Как масштабировать сервис?

Измените `replicas` в `values.yaml` или через Helm:

```bash
helm upgrade gestalt ./k8s/helm/gestalt --set geshtalt.replicas=2 -n gestalt
```

### Как обновить секреты?

Через GitHub Secrets (если используете External Secrets) или через Helm:

```bash
helm upgrade gestalt ./k8s/helm/gestalt \
  --set secrets.redisPassword="new-password" \
  -n gestalt
```

