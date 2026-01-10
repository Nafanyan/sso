# SSO (Single Sign-On) Service

Микросервис для аутентификации и авторизации пользователей с использованием gRPC API.

## Описание

SSO сервис предоставляет централизованную систему аутентификации, позволяющую пользователям регистрироваться и входить в различные приложения через единую точку входа. Сервис использует JWT токены для аутентификации и поддерживает работу с несколькими приложениями.

## Основные возможности

- ✅ Регистрация новых пользователей
- ✅ Аутентификация пользователей (логин)
- ✅ Генерация JWT токенов для авторизации
- ✅ Поддержка множественных приложений
- ✅ Хеширование паролей с использованием bcrypt
- ✅ Валидация входных данных
- ✅ Graceful shutdown
- ✅ Структурированное логирование

## Технологический стек

- **Go 1.24+**
- **gRPC** - для API коммуникации
- **SQLite** - для хранения данных
- **JWT** - для токенов аутентификации
- **bcrypt** - для хеширования паролей
- **golang-migrate** - для миграций базы данных

## Структура проекта

```
sso/
├── cmd/
│   ├── migrator/         # Миграции БД
│   └── sso/              # Основное приложение
├── config/               # Конфигурационные файлы
├── internal/
│   ├── app/              # Инициализация приложения
│   ├── config/           # Конфигурация
│   ├── domain/
│   │   └── models/       # Модели данных
│   ├── grpc/
│   │   └── auth/         # gRPC handlers
│   ├── lib/
│   │   ├── jwt/          # JWT токены
│   │   └── logger/       # Логирование
│   ├── services/
│   │   └── auth/         # Бизнес-логика аутентификации
│   └── storage/
│       └── sqlite/       # Реализация хранилища
├── migrations/           # SQL миграции
├── tests/                # Интеграционные тесты
└── storage/              # Директория для БД (SQLite)
```

## Установка и запуск

### Требования

- Go 1.24 или выше
- SQLite3

### Установка зависимостей

```bash
go mod download
```

### Конфигурация

Создайте файл конфигурации `config/config_local.yaml`:

```yaml
env: "local"
storage_path: "./storage/sso.db"
grpc:
  port: 8080
  timeout: 10s
token_ttl: 1h
```

Или используйте переменную окружения:
```bash
export CONFIG_PATH=./config/config_local.yaml
```

### Запуск миграций

```bash
go run cmd/migrator/main.go
```

### Запуск приложения

```bash
go run cmd/sso/main.go -config-path ./config/config_local.yaml
```

Или через переменную окружения:
```bash
export CONFIG_PATH=./config/config_local.yaml
go run cmd/sso/main.go
```

## API

### Регистрация пользователя

**Endpoint:** `Auth.Register`

**Request:**
```protobuf
message RegisterRequest {
  string email = 1;
  string password = 2;
}
```

**Response:**
```protobuf
message RegisterResponse {
  int64 user_id = 1;
}
```

**Пример использования:**
```go
req := &ssov1.RegisterRequest{
    Email:    "user@example.com",
    Password: "securepassword",
}
resp, err := authClient.Register(ctx, req)
```

### Аутентификация (логин)

**Endpoint:** `Auth.Login`

**Request:**
```protobuf
message LoginRequest {
  string email = 1;
  string password = 2;
  int32 app_id = 3;
}
```

**Response:**
```protobuf
message LoginResponse {
  string token = 1;
}
```

**Пример использования:**
```go
req := &ssov1.LoginRequest{
    Email:    "user@example.com",
    Password: "securepassword",
    AppId:    1,
}
resp, err := authClient.Login(ctx, req)
// resp.Token содержит JWT токен
```

## Валидация

- **Email:** обязательное поле, длина от 3 до 254 символов
- **Password:** обязательное поле, минимум 8 символов
- **App ID:** обязательное поле, должен существовать в базе данных

## Безопасность

- Пароли хешируются с использованием bcrypt
- JWT токены подписываются секретом приложения
- Пароли не хранятся в открытом виде
- Валидация всех входных данных

## Тестирование

Запуск интеграционных тестов:

```bash
go test ./tests/...
```

## Логирование

Приложение поддерживает три режима логирования:

- **local** - цветной вывод для разработки
- **dev** - JSON формат с уровнем Debug
- **prod** - JSON формат с уровнем Info

Уровень логирования настраивается через поле `env` в конфигурации.

## Graceful Shutdown

Приложение поддерживает корректное завершение работы:
- Обработка сигналов SIGTERM и SIGINT
- Таймаут завершения: 10 секунд
- Корректное закрытие соединений с базой данных

## TODO

Список задач для дальнейшей разработки находится в файле [TODO.md](./TODO.md).