# Saturnita Pulse

Для локального запуска см. [README проекта](../README.md). Для эксплуатации доступны [инструкция по развёртыванию и восстановлению](production.md), [контракт API кабинета](../openapi/identity.json), [контракт API продукта](../openapi/product.json) и [миграции PostgreSQL](../backend/migrations/).

Исходный проект этапа 1 сохранён в [проектной SQL-схеме](schema.sql) и [проектном OpenAPI](openapi.json).

Документы этапа:

- [architecture.md](architecture.md) — требования, границы модулей, структура проекта, решения и допущения;
- [requirements.md](requirements.md) — трассировка функциональных и нефункциональных требований, риски и границы MVP;
- [database.md](database.md) — PostgreSQL-схема, ключи, индексы, ограничения и хранение данных;
- [diagrams.md](diagrams.md) — ERD и C4 Container/Component в Mermaid;
- [api.md](api.md) — REST API, DTO, ошибки, пагинация и авторизация;
- [routing-monitoring.md](routing-monitoring.md) — маршрутизация host/path, redirect flow, worker и SSRF-защита;
- [frontend.md](frontend.md) — дизайн-система, компоненты и информационная архитектура;
- [backlog.md](backlog.md) — задачи для GitHub Issues, зависимости и критерии приёмки.

## Граница этапа

На этом этапе не создаются Go/React-приложения, миграции, Docker-конфигурация, не выполняется deployment и не меняются DNS-записи. Документы задают контракты для последующих этапов, но начало этапа 2 требует отдельного подтверждения.

## Краткий итог

- Архитектурный стиль: модульный монолит, один deployable backend, один React SPA и PostgreSQL.
- API: same-origin `/api/v1` на `saturnita.ru`; collector также доступен публично, redirect определяется по нормализованному `Host`.
- Фоновые проверки: ограниченный пул goroutine внутри backend для MVP; выдача работ через PostgreSQL lease и `FOR UPDATE SKIP LOCKED`.
- Данные: UUID для внутренних идентификаторов, UTC `timestamptz`, явные SQL-миграции и tenant-фильтрация по `user_id`.
- Безопасность мониторинга: разрешены только HTTP(S), проверяется каждый DNS-адрес и фактический dial target, redirect по умолчанию запрещён.
- DNS/TLS: только проект конфигурации wildcard и Caddy On-Demand TLS; никаких реальных изменений.

