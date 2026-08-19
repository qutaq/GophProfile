# GophProfile

Микросервис управления аватарками пользователей.

Стек: **Go**, **Chi**, **PostgreSQL**, **MinIO**, **RabbitMQ**, **golang-migrate**, Docker Compose. Наблюдаемость: **OpenTelemetry**, **Prometheus**, **Jaeger**, **Grafana Loki**.

## Структура

```
cmd/server/              # HTTP API
cmd/worker/              # асинхронная обработка изображений
cmd/migrate/             # применение SQL-миграций
cmd/check/               # проверка Postgres / MinIO / RabbitMQ
internal/config/         # конфигурация из env
internal/infra/          # клиенты инфраструктуры
internal/observability/  # трейсы, метрики, slog
migrations/              # SQL-миграции (golang-migrate)
web/                     # веб-интерфейс
deploy/observability/    # Prometheus, Loki, Grafana, Alertmanager
```

## Быстрый старт (Docker)

Одной командой поднимаются Postgres, MinIO, RabbitMQ, migrate, server и worker:

```bash
docker compose up -d --build
# или
task up
```

С наблюдаемостью (Jaeger, Prometheus, Loki, Grafana, Alertmanager):

```bash
docker compose --profile obs up -d --build
# или
task up:obs
```

Проверка:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/web/upload
curl http://localhost:8080/metrics
```

Остановка: `task down` · логи: `task logs` · логи obs: `task logs:obs`

### Локальный запуск без контейнеров приложения

```bash
cp .env.example .env
task deps
task up:infra      # только Postgres/MinIO/RabbitMQ
task migrate
task run-server    # терминал 1
task run-worker    # терминал 2
```

Список задач: `task --list`.

Веб-интерфейс:
- Upload: http://localhost:8080/web/upload
- Gallery: http://localhost:8080/web/gallery/demo-user

### UI сервисов

| Сервис | URL | Доступ |
|--------|-----|--------|
| API / Web | http://localhost:8080 | |
| Grafana | http://localhost:3000 | `admin` / `admin` |
| Jaeger | http://localhost:16686 | |
| Prometheus | http://localhost:9090 | |
| Alertmanager | http://localhost:9093 | |
| Loki | http://localhost:3100 | |
| MinIO Console | http://localhost:9001 | `minioadmin` / `minioadmin` |
| RabbitMQ Management | http://localhost:15672 | `guest` / `guest` |

Grafana datasources (Prometheus, Loki, Jaeger) поднимаются автоматически. Дашборд **GophProfile / Service overview**: RED, resources, business KPIs, worker, error logs. С лога по `trace_id` — переход в Jaeger (кнопка **Explore Jaeger** на дашборде). Loki Explore: `{service="gophprofile-server"} | json | trace_id="<id>"`.

### API

```bash
# health
curl http://localhost:8080/health

# upload
curl -X POST http://localhost:8080/api/v1/avatars \
  -H "X-User-ID: user-1" \
  -F "file=@./photo.jpg;type=image/jpeg"

# get / metadata / list / delete
curl http://localhost:8080/api/v1/avatars/{id} --output avatar.jpg
curl "http://localhost:8080/api/v1/avatars/{id}?size=100x100" --output thumb.jpg
curl http://localhost:8080/api/v1/avatars/{id}/metadata
curl http://localhost:8080/api/v1/users/user-1/avatars
curl -X DELETE http://localhost:8080/api/v1/avatars/{id} -H "X-User-ID: user-1"
```

Worker (миниатюры и удаление из S3):

```bash
task run-worker
```

## Конфигурация

Все параметры задаются через переменные окружения. См. [`.env.example`](.env.example).

| Группа        | Примеры переменных                                      |
|---------------|---------------------------------------------------------|
| HTTP          | `HTTP_ADDR`, `HTTP_READ_TIMEOUT`                        |
| DB            | `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` |
| MinIO         | `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET` |
| RabbitMQ      | `RABBITMQ_URL`, `RABBITMQ_EXCHANGE`                     |
| Upload        | `UPLOAD_MAX_SIZE_BYTES` (по умолчанию 10MB)             |
| Observability | `OTEL_ENABLED`, `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `LOG_LEVEL`, `METRICS_PATH`, `METRICS_ADDR` |

Для локального запуска приложения на хосте при поднятом `task up:obs` используйте `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317` (уже в `.env.example`).

## Зафиксированный стек

- HTTP: [go-chi/chi](https://github.com/go-chi/chi)
- Object storage: [minio-go](https://github.com/minio/minio-go)
- Broker: [amqp091-go](https://github.com/rabbitmq/amqp091-go) (RabbitMQ)
- Migrations: [golang-migrate](https://github.com/golang-migrate/migrate)
- Tracing: OpenTelemetry → OTLP → Jaeger
- Metrics: Prometheus (`/metrics` на server `:8080` и worker `:9091`)
- Logs: slog JSON → Promtail → Loki

## Тесты и качество

```bash
task test          # все тесты
task cover         # ./internal/... и порог покрытия >= 50%
task lint:install  # один раз установить golangci-lint
task lint          # статический анализ

# интеграционные тесты репозитория/S3 требуют поднятой infra:
task up
go test ./internal/repository/... -count=1 -v
```
