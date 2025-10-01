# Kubernetes Installer

Автоматизированная установка Kubernetes control plane на Go с поддержкой GitHub Actions.

## Особенности

- ✅ Полностью автоматизированная установка Kubernetes
- ✅ Чистый Go код без внешних зависимостей
- ✅ Модульная архитектура
- ✅ Поддержка GitHub Actions CI/CD
- ✅ Подробное логирование
- ✅ Генерация сертификатов
- ✅ Настройка CNI (Container Network Interface)
- ✅ Интеграция containerd

## Требования

- Ubuntu/Debian Linux
- Go 1.22 или выше
- Root права (sudo)
- Минимум 2GB RAM
- 20GB свободного места на диске

## Быстрый старт

### 1. Клонировать репозиторий

```bash
git clone https://github.com/yourname/k8s-installer.git
cd k8s-installer
```

### 2. Установить зависимости

```bash
make deps
```

### 3. Собрать проект

```bash
make build
```

### 4. Запустить установку

```bash
make run
```

Или с подробным выводом:

```bash
make run-verbose
```

## Использование

### Команды Make

```bash
make help              # Показать все доступные команды
make build             # Собрать бинарный файл
make install           # Установить в /usr/local/bin
make run              # Собрать и запустить
make run-verbose      # Запустить с подробным выводом
make test             # Запустить тесты
make test-coverage    # Запустить тесты с покрытием
make clean            # Удалить артефакты сборки и установки K8s
make verify           # Проверить статус кластера
make create-deployment # Создать тестовый deployment nginx
```

### Ручной запуск

```bash
# Базовая установка
sudo ./build/k8s-installer

# С параметрами
sudo ./build/k8s-installer -k8s-version v1.30.0 -verbose

# Доступные флаги:
#   -k8s-version string    Версия Kubernetes (default "v1.30.0")
#   -skip-download         Пропустить загрузку бинарных файлов
#   -skip-verify          Пропустить проверку
#   -verbose              Подробный вывод
```

## Структура проекта

```
k8s-installer/
├── cmd/
│   └── installer/          # Точка входа приложения
│       └── main.go
├── internal/
│   ├── installer/          # Основная логика установки
│   │   ├── installer.go    # Главный контроллер
│   │   ├── directories.go  # Создание директорий
│   │   ├── downloads.go    # Загрузка бинарных файлов
│   │   ├── certificates.go # Генерация сертификатов
│   │   ├── configs.go      # Создание конфигураций
│   │   └── verify.go       # Проверка установки
│   ├── services/           # Сервисы Kubernetes
│   │   ├── manager.go      # Менеджер сервисов
│   │   ├── etcd.go         # Etcd сервис
│   │   ├── apiserver.go    # API Server
│   │   ├── containerd.go   # Container runtime
│   │   ├── kubelet.go      # Kubelet
│   │   ├── scheduler.go    # Scheduler
│   │   └── controller.go   # Controller Manager
│   └── utils/              # Утилиты
│       ├── network.go      # Сетевые функции
│       └── downloader.go   # Загрузчик файлов
├── .github/
│   └── workflows/
│       └── k8s-setup.yml   # GitHub Actions workflow
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## GitHub Actions

Проект включает готовый workflow для автоматической установки и тестирования в GitHub Actions:

```yaml
# .github/workflows/k8s-setup.yml
name: Kubernetes Setup

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]
  workflow_dispatch:
```

Workflow автоматически:
- Собирает проект
- Запускает тесты
- Устанавливает Kubernetes
- Проверяет работоспособность кластера
- Создает тестовый deployment
- Загружает логи и артефакты

## Проверка установки

После успешной установки:

```bash
# Проверить ноды
./kubebuilder/bin/kubectl get nodes

# Проверить поды
./kubebuilder/bin/kubectl get pods -A

# Проверить компоненты
./kubebuilder/bin/kubectl get componentstatuses

# Создать тестовый deployment
./kubebuilder/bin/kubectl create deployment nginx --image=nginx:latest

# Проверить deployment
./kubebuilder/bin/kubectl get deployments
./kubebuilder/bin/kubectl get pods
```

## Логи

Все логи сервисов находятся в `/var/log/kubernetes/`:

```bash
/var/log/kubernetes/
├── etcd.log
├── apiserver.log
├── containerd.log
├── scheduler.log
├── kubelet.log
└── controller-manager.log
```

Просмотр логов:

```bash
# Просмотр всех логов
sudo tail -f /var/log/kubernetes/*.log

# Просмотр конкретного сервиса
sudo tail -f /var/log/kubernetes/apiserver.log
```

## Очистка

Для полной очистки установки:

```bash
make clean
```

Для очистки только логов:

```bash
make clean-logs
```

## Компоненты

### Установленные компоненты:

- **etcd** - распределенное хранилище key-value
- **kube-apiserver** - API сервер Kubernetes
- **kube-controller-manager** - менеджер контроллеров
- **kube-scheduler** - планировщик подов
- **kubelet** - агент на ноде
- **containerd** - container runtime
- **CNI plugins** - сетевые плагины

### Версии по умолчанию:

- Kubernetes: v1.30.0
- Containerd: 2.0.5
- Runc: v1.2.6
- CNI Plugins: v1.6.2

## Безопасность

⚠️ **Важно**: Эта установка предназначена для разработки и обучения. Не используйте в продакшене!

- Используются самоподписанные сертификаты
- Токен авторизации захардкожен
- Режим авторизации: AlwaysAllow
- Отключена TLS верификация

## Устранение неполадок

### API Server не запускается

```bash
# Проверить логи
sudo tail -100 /var/log/kubernetes/apiserver.log

# Проверить etcd
sudo tail -100 /var/log/kubernetes/etcd.log
```

### Kubelet не может создать под

```bash
# Проверить статус containerd
sudo tail -100 /var/log/kubernetes/containerd.log

# Проверить CNI конфигурацию
cat /etc/cni/net.d/10-mynet.conf
```

### Недостаточно прав

Убедитесь, что запускаете с sudo:

```bash
sudo make run
```

## Разработка

### Запуск тестов

```bash
# Все тесты
make test

# Тесты с покрытием
make test-coverage
```

### Форматирование кода

```bash
make fmt
```

### Проверка кода

```bash
make vet
```

## Вклад в проект

1. Fork репозитория
2. Создайте feature branch (`git checkout -b feature/amazing-feature`)
3. Commit изменения (`git commit -m 'Add amazing feature'`)
4. Push в branch (`git push origin feature/amazing-feature`)
5. Откройте Pull Request

## Лицензия

MIT License

## Авторы

Основано на [mastering-k8s](https://github.com/den-vasyliev/mastering-k8s) от den-vasyliev

## Ссылки

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Containerd](https://containerd.io/)
- [CNI Specification](https://github.com/containernetworking/cni)