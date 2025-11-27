# Установка k3s (легковесный Kubernetes)

k3s - это легковесный Kubernetes, идеально подходящий для одного сервера. Занимает ~50MB и требует минимум ресурсов.

## Установка на Ubuntu/Debian

```bash
# Установка k3s
curl -sfL https://get.k3s.io | sh -

# Проверка статуса
systemctl status k3s

# Получение kubeconfig (для использования kubectl)
mkdir -p ~/.kube
sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config
sudo chown $USER ~/.kube/config

# Проверка кластера
kubectl cluster-info
kubectl get nodes
```

## Настройка для удаленного доступа (опционально)

Если нужно подключаться с других серверов:

```bash
# Отредактируйте /etc/rancher/k3s/k3s.yaml
# Замените 127.0.0.1 на IP сервера или 0.0.0.0
sudo sed -i 's/127.0.0.1/0.0.0.0/g' /etc/rancher/k3s/k3s.yaml

# Перезапустите k3s
sudo systemctl restart k3s
```

## Установка Helm (если еще не установлен)

```bash
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

## Проверка готовности

```bash
# Проверка нод
kubectl get nodes

# Проверка системных подов
kubectl get pods -n kube-system

# Проверка Helm
helm version
```

## Развертывание приложения

После установки k3s можно развернуть приложение:

```bash
cd /root/gestalt

# Создайте namespace
kubectl create namespace gestalt

# Установите Helm чарт
helm install gestalt ./k8s/helm/gestalt \
  --namespace gestalt \
  --set domain.autoDetect=true \
  --set domain.certbotEmail="your-email@example.com"
```

## Удаление k3s (если нужно)

```bash
/usr/local/bin/k3s-uninstall.sh
```

## Преимущества k3s

- ✅ Легковесный (~50MB)
- ✅ Быстрая установка
- ✅ Минимальные требования к ресурсам
- ✅ Полная совместимость с Kubernetes API
- ✅ Идеален для одного сервера
- ✅ Встроенный Traefik (Ingress Controller)

## Альтернативы

Если k3s не подходит, можно использовать:
- **Minikube** - для локальной разработки
- **Kubeadm** - для полноценного кластера (требует больше ресурсов)
- **MicroK8s** - еще один легковесный вариант от Canonical

