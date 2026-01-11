# SSO (Single Sign-On) Service

Микросервис для аутентификации и авторизации пользователей с использованием gRPC API.

## Описание

SSO сервис предоставляет централизованную систему аутентификации, позволяющую пользователям регистрироваться и входить в различные приложения через единую точку входа. Сервис использует JWT токены для аутентификации и поддерживает работу с несколькими приложениями. Система включает контроль доступа на уровне связей пользователь-приложение, что позволяет гибко управлять правами доступа к различным приложениям.

## Основные возможности

- ✅ Регистрация новых пользователей
- ✅ Аутентификация пользователей (логин)
- ✅ Генерация JWT токенов для авторизации
- ✅ Валидация JWT токенов
- ✅ Управление доступом пользователей к приложениям (grant/revoke access)
- ✅ Поддержка множественных приложений
- ✅ Контроль доступа на уровне user-app связей
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
├── migrations/           # SQL миграции (основные)
├── tests/                # Интеграционные тесты
│   ├── migrations/       # SQL миграции для тестов
│   └── suite/            # Test suite
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
go run ./cmd/migrator --storage-path=./storage/sso.db --migrations-path=./migrations
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
  string app_code = 3;
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
    AppCode:  "web",
}
resp, err := authClient.Login(ctx, req)
// resp.Token содержит JWT токен
```

**Примечание:** Для успешного входа пользователь должен иметь доступ к указанному приложению (через таблицу `user_app`).

### Валидация токена

**Endpoint:** `Auth.Validate`

**Request:**
```protobuf
message ValidateTokenRequest {
  string token = 1;
  string app_code = 2;
}
```

**Response:**
```protobuf
message ValidateTokenResponse {
  string email = 1;
}
```

**Пример использования:**
```go
req := &ssov1.ValidateTokenRequest{
    Token:   "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    AppCode: "web",
}
resp, err := authClient.Validate(ctx, req)
// resp.Email содержит email пользователя, если токен валиден
```

### Предоставление доступа

**Endpoint:** `Auth.AllowAccess` или `Auth.GrantAccess`

**Request:**
```protobuf
message AllowAccessRequest {
  string email = 1;
  string app_code = 2;
}
```

**Response:**
```protobuf
message AllowAccessResponse {
  string app_code = 1;
}
```

**Пример использования:**
```go
req := &ssov1.AllowAccessRequest{
    Email:   "user@example.com",
    AppCode: "web",
}
resp, err := authClient.AllowAccess(ctx, req)
```

### Отзыв доступа

**Endpoint:** `Auth.RevokeAccess`

**Request:**
```protobuf
message RevokeAccessRequest {
  string email = 1;
  string app_code = 2;
}
```

**Response:**
```protobuf
message RevokeAccessResponse {
  string app_code = 1;
}
```

**Пример использования:**
```go
req := &ssov1.RevokeAccessRequest{
    Email:   "user@example.com",
    AppCode: "web",
}
resp, err := authClient.RevokeAccess(ctx, req)
```

## Валидация

- **Email:** обязательное поле, длина от 3 до 254 символов
- **Password:** обязательное поле, минимум 8 символов
- **App Code:** обязательное поле, должен существовать в базе данных
- **Token:** обязательное поле для валидации, должен быть валидным JWT токеном

## Безопасность

- Пароли хешируются с использованием bcrypt
- JWT токены подписываются секретом приложения
- Пароли не хранятся в открытом виде
- Валидация всех входных данных
- Контроль доступа на уровне приложений (user-app связи)
- Проверка прав доступа при логине и валидации токена
- Каждое приложение имеет уникальный секрет для подписи токенов

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