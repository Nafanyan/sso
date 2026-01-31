# Интеграция с SSO-сервисом

Руководство по интеграции стороннего приложения с SSO-сервисом аутентификации.

## Содержание

- [Рекомендуемая архитектура](#рекомендуемая-архитектура)
- [Регистрация приложения](#регистрация-приложения)
- [Подключение gRPC-клиента](#подключение-grpc-клиента)
- [API (контракты)](#api-контракты)
- [JWT токен](#jwt-токен)
- [Обработка ошибок](#обработка-ошибок)

---

## Рекомендуемая архитектура

Схема с тремя клиентами (web, mobile, desktop), единым backend и SSO:

```
                    ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
                    │    Web      │  │   Mobile    │  │   Desktop   │
                    │   клиент    │  │   клиент    │  │   клиент    │
                    └──────┬──────┘  └──────┬──────┘  └──────┬──────┘
                           │                │                │
                           │    HTTP/HTTPS (token + app_code)
                           └────────────────┼────────────────┘
                                            │
                                            ▼
                                    ┌───────────────┐
                                    │    Backend    │
                                    │  вашего API   │
                                    └───────┬───────┘
                                            │
                                            │  gRPC
                                            ▼
                                    ┌───────────────┐
                                    │      SSO      │
                                    └───────────────┘
```

### Поток регистрации

Неважно, с какого клиента (web, mobile или desktop) пользователь регистрируется — запрос всегда идёт через backend:

```
Клиент  ──►  Backend  ──►  SSO (Register)
  │              │              │
  │              │  ◄───────────┘  user_id
  │  ◄───────────┘
```

1. Пользователь заполняет форму на любом клиенте.
2. Клиент отправляет `email` и `password` на backend.
3. Backend перенаправляет запрос в SSO (`Auth.Register`).
4. SSO регистрирует пользователя, возвращает `user_id`.
5. Backend возвращает результат клиенту.

### Поток входа (логин)

Пример: пользователь логинится с web-клиента.

```
Web  ──►  Backend  ──►  SSO (Login, app_code="web")
  │           │                    │
  │           │  ◄─────────────────┘  JWT токен
  │  ◄────────┘
```

1. Пользователь вводит email и пароль на клиенте.
2. Клиент отправляет данные на backend.
3. Backend вызывает SSO: `Auth.Login` с `app_code`, соответствующим клиенту (`web`, `mobile` или `desktop`).
4. SSO возвращает JWT токен.
5. Backend отдаёт токен клиенту.
6. Клиент сохраняет токен (cookie, localStorage, secure storage и т.п.).

### Последующие запросы к API

Клиент при каждом запросе передаёт **токен** и **app_code**:

```
Клиент  ──►  Backend  ──►  SSO (Validate, token, app_code)
  │              │                    │
  │              │  ◄─────────────────┘  email / ошибка
  │              │
  │              │  Обрабатывает запрос, если токен валиден
  │  ◄───────────┘
```

1. Клиент добавляет к запросу токен (например, заголовок `Authorization: Bearer <token>`) и `app_code` (заголовок `X-App-Code` или в теле/query).
2. Backend понимает, откуда идёт трафик (web/mobile/desktop) и вызывает `Auth.Validate` с токеном и `app_code`.
3. SSO проверяет токен и права доступа, возвращает `email` или ошибку.
4. Backend обрабатывает запрос и возвращает ответ клиенту.

### Независимость сессий: отдельный токен на каждый клиент

У каждого клиента (web, mobile, desktop) — свой токен. Токены **не перетирают** друг друга при входах с разных устройств.

| Клиент  | app_code  | Токен      |
|---------|-----------|------------|
| Web     | `web`     | токен_1    |
| Mobile  | `mobile`  | токен_2    |
| Desktop | `desktop` | токен_3    |

- Пользователь вошёл с web → получил токен для `web`.
- Позже вошёл с mobile → получил отдельный токен для `mobile`.
- Оба токена действуют параллельно.

**Выход из одного клиента не влияет на другие:** если пользователь вышел из web (удалил токен на своей стороне), сессии на mobile и desktop сохраняются. Backend при валидации проверяет конкретный токен и `app_code`, сессии изолированы.

---

## Регистрация приложения

Перед интеграцией в SSO должны быть зарегистрированы приложения для каждого клиента. Администратор добавляет записи в таблицу `apps`:

| code     | Описание        | secret (пример)  |
|----------|-----------------|------------------|
| `web`    | Веб-клиент      | `web-secret-key` |
| `mobile` | Мобильное приложение | `mobile-secret-key` |
| `desktop`| Десктоп-приложение  | `desktop-secret-key` |

> **Важно:** Backend общается с SSO по gRPC и вызывает `Validate` — секреты приложений хранятся только в SSO. Backend не должен хранить секреты клиентских приложений.

---

## Подключение gRPC-клиента

Backend подключается к SSO по gRPC:

```go
import (
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    ssov1 "github.com/Nafanyan/sso-proto/gen/go/sso"
)

conn, err := grpc.Dial("localhost:8080",
    grpc.WithTransportCredentials(insecure.NewCredentials()))
if err != nil {
    return err
}
defer conn.Close()

authClient := ssov1.NewAuthClient(conn)
```

---

## API (контракты)

### Register — регистрация пользователя

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

**Пример:**
```go
resp, err := authClient.Register(ctx, &ssov1.RegisterRequest{
    Email:    "user@example.com",
    Password: "securepassword",
})
```

---

### Login — аутентификация (вход)

**Endpoint:** `Auth.Login`

**Request:**
```protobuf
message LoginRequest {
  string email = 1;
  string password = 2;
  string app_code = 3;  // "web", "mobile" или "desktop"
}
```

**Response:**
```protobuf
message LoginResponse {
  string token = 1;
}
```

**Пример:**
```go
resp, err := authClient.Login(ctx, &ssov1.LoginRequest{
    Email:    "user@example.com",
    Password: "securepassword",
    AppCode:  "web",  // или "mobile", "desktop"
})
token := resp.GetToken()
```

Для успешного входа пользователь должен иметь доступ к указанному приложению (через `user_app`). При необходимости доступ выдают через `AllowAccess`.

---

### Validate — валидация токена

**Endpoint:** `Auth.Validate`

**Request:**
```protobuf
message ValidateTokenRequest {
  string token = 1;
  string app_code = 2;  // должен соответствовать app_code токена
}
```

**Response:**
```protobuf
message ValidateTokenResponse {
  string email = 1;
}
```

**Пример:**
```go
resp, err := authClient.Validate(ctx, &ssov1.ValidateTokenRequest{
    Token:   tokenFromClient,
    AppCode: "web",
})
if err != nil {
    // токен невалиден или истёк
    return err
}
userEmail := resp.GetEmail()
```

---

### AllowAccess / GrantAccess — выдача доступа

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

Используется в админ-панели для выдачи пользователю доступа к приложению.

---

### RevokeAccess — отзыв доступа

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

После отзыва доступ к приложению отзывается, существующие токены перестают проходить валидацию.

---

### Валидация полей

- **Email:** обязательно, длина от 3 до 254 символов
- **Password:** обязательно, минимум 8 символов
- **App Code:** обязательно, должен существовать в БД SSO
- **Token:** обязательно при вызове `Validate`

---

## JWT токен

Структура claims:

| Claim     | Тип    | Описание                      |
|-----------|--------|-------------------------------|
| `uid`     | int64  | ID пользователя               |
| `email`   | string | Email пользователя            |
| `app_code`| string | Код приложения (web/mobile/desktop) |
| `exp`     | int64  | Unix timestamp истечения      |

Токен подписывается секретом приложения (HMAC-SHA256). Время жизни задаётся конфигурацией SSO (`token_ttl`).

---

## Обработка ошибок

| Код gRPC        | Описание                                                        |
|-----------------|-----------------------------------------------------------------|
| `InvalidArgument` | Невалидные данные (пустой email, короткий пароль и т.п.)      |
| `AlreadyExists`   | Пользователь уже зарегистрирован                               |
| `Unauthenticated` | Неверные учётные данные, истёкший/неверный токен, доступ отозван |
| `Internal`        | Внутренняя ошибка SSO                                          |

**Сообщения об ошибках:**

- `email is required` / `password is required` / `app_code is required` — не заполнены обязательные поля
- `invalid email or password` — неверный email или пароль
- `Access denied` — у пользователя нет доступа к приложению
- `Token is expired` — токен истёк
- `Token is invalid` — токен повреждён или неверный
- `user already exists` — email уже зарегистрирован
