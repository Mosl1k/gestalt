# Kubernetes Deployment

Этот проект настроен для развертывания в Kubernetes с использованием Helm чартов и ArgoCD.

## Структура

```
k8s/
├── helm/
│   └── gestalt/          # Helm чарт для всего приложения
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/    # Kubernetes манифесты
├── argocd/
│   ├── application.yaml  # ArgoCD Application
│   └── external-secrets.yaml  # External Secrets Operator (опционально)
└── README.md
```

## Предварительные требования

1. **Kubernetes кластер** (версия 1.24+)
2. **Helm** (версия 3.0+)
3. **ArgoCD** (опционально, для GitOps)
4. **External Secrets Operator** (опционально, для синхронизации секретов из GitHub)

## Установка

### 1. Установка ArgoCD (рекомендуется)

```bash
kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

### 2. Установка External Secrets Operator (опционально)

```bash
helm repo add external-secrets https://charts.external-secrets.io
helm install external-secrets external-secrets/external-secrets -n external-secrets-system --create-namespace
```

### 3. Настройка секретов

#### Вариант A: Через GitHub Secrets (рекомендуется с External Secrets)

1. Добавьте секреты в GitHub Repository Secrets:
   - `REDIS_PASSWORD`
   - `SESSION_SECRET`
   - `YANDEX_CLIENT_ID`
   - `YANDEX_CLIENT_SECRET`
   - `TELEGRAM_TOKEN`
   - `SERVICE_USER_ID`

2. Примените External Secrets манифест:
```bash
kubectl apply -f k8s/argocd/external-secrets.yaml
```

#### Вариант B: Через Helm values

Создайте файл `k8s/helm/gestalt/values-production.yaml`:

```yaml
secrets:
  redisPassword: "your-redis-password"
  sessionSecret: "your-session-secret"
  yandexClientId: "your-client-id"
  yandexClientSecret: "your-client-secret"
  telegramToken: "your-telegram-token"
  serviceUserId: "your-service-user-id"
```

И используйте его при установке:
```bash
helm install gestalt ./k8s/helm/gestalt -f k8s/helm/gestalt/values-production.yaml
```

#### Вариант C: Через kubectl

```bash
kubectl create secret generic gestalt-secrets \
  --from-literal=REDIS_PASSWORD='your-password' \
  --from-literal=SESSION_SECRET='your-secret' \
  --from-literal=YANDEX_CLIENT_ID='your-id' \
  --from-literal=YANDEX_CLIENT_SECRET='your-secret' \
  --from-literal=TELEGRAM_TOKEN='your-token' \
  --from-literal=SERVICE_USER_ID='your-user-id' \
  -n gestalt
```

### 4. Развертывание через ArgoCD

```bash
kubectl apply -f k8s/argocd/application.yaml
```

ArgoCD автоматически синхронизирует приложение при изменениях в репозитории.

### 5. Развертывание через Helm напрямую

```bash
helm repo add gestalt ./k8s/helm/gestalt
helm install gestalt ./k8s/helm/gestalt \
  --namespace gestalt \
  --create-namespace \
  --set secrets.redisPassword="$REDIS_PASSWORD" \
  --set secrets.sessionSecret="$SESSION_SECRET" \
  --set secrets.yandexClientId="$YANDEX_CLIENT_ID" \
  --set secrets.yandexClientSecret="$YANDEX_CLIENT_SECRET" \
  --set secrets.telegramToken="$TELEGRAM_TOKEN" \
  --set secrets.serviceUserId="$SERVICE_USER_ID"
```

## Обновление

### Через ArgoCD

ArgoCD автоматически обнаружит изменения в репозитории и синхронизирует их. Можно также принудительно синхронизировать через UI или CLI:

```bash
argocd app sync gestalt
```

### Через Helm

```bash
helm upgrade gestalt ./k8s/helm/gestalt \
  --namespace gestalt \
  -f k8s/helm/gestalt/values-production.yaml
```

## Настройка ресурсов

Редактируйте `k8s/helm/gestalt/values.yaml` для настройки ресурсов под ваш сервер:

```yaml
geshtalt:
  resources:
    requests:
      memory: "128Mi"
      cpu: "100m"
    limits:
      memory: "256Mi"
      cpu: "200m"
  replicas: 1
```

## Персистентное хранилище

Redis использует PersistentVolume для хранения данных. Убедитесь, что в кластере настроен StorageClass:

```yaml
redis:
  persistence:
    enabled: true
    size: 1Gi
    storageClass: ""  # Использует default StorageClass
```

## Мониторинг и логи

```bash
# Логи всех подов
kubectl logs -l app.kubernetes.io/name=gestalt -n gestalt --tail=100

# Логи конкретного сервиса
kubectl logs -l app.kubernetes.io/component=geshtalt -n gestalt

# Статус подов
kubectl get pods -n gestalt

# Статус сервисов
kubectl get svc -n gestalt
```

## Удаление

```bash
# Через ArgoCD
kubectl delete -f k8s/argocd/application.yaml

# Через Helm
helm uninstall gestalt -n gestalt
```

## Миграция с Docker Compose

1. Экспортируйте данные из Redis:
```bash
kubectl exec -it <redis-pod> -n gestalt -- redis-cli --rdb /tmp/dump.rdb
```

2. Импортируйте в новый Redis:
```bash
kubectl cp dump.rdb <redis-pod>:/data/dump.rdb -n gestalt
kubectl exec -it <redis-pod> -n gestalt -- redis-cli --rdb /data/dump.rdb
```

## Troubleshooting

### Поды не запускаются

```bash
kubectl describe pod <pod-name> -n gestalt
kubectl logs <pod-name> -n gestalt
```

### Проблемы с секретами

```bash
kubectl get secrets -n gestalt
kubectl describe secret gestalt-secrets -n gestalt
```

### Проблемы с сетью

```bash
kubectl get svc -n gestalt
kubectl get endpoints -n gestalt
```

## Дополнительные ресурсы

- [Helm Documentation](https://helm.sh/docs/)
- [ArgoCD Documentation](https://argo-cd.readthedocs.io/)
- [External Secrets Operator](https://external-secrets.io/)

