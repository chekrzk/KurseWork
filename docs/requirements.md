# Анализ и трассировка требований

## 1. Функциональный охват

| Область SPEC | Обязательное поведение | Архитектурное покрытие | Основной риск |
|---|---|---|---|
| Identity | регистрация, login/logout, профиль, пароль, сессии, private routes | `identity`, hashed server sessions, CSRF, owner-scoped SQL | credential stuffing, session leakage |
| Links | CRUD, auto/custom slug, expiry/status/quota/UTM, archive/delete | `links`, DB uniqueness, atomic quota transaction | redirect latency и race quota |
| Redirect | host+slug, click record, 302, target query сохранён | public handler + `domains/links/linkanalytics` | host spoofing, cycle, analytics failure |
| Domains | system domain, wildcard subdomains, verified custom domains | `domains`, TXT challenge, Caddy allow endpoint | certificate abuse, неверная DNS-модель |
| Link analytics | clicks, periods, Приблизительные уникальные посетители, devices/sources/UTM/country | append-only clicks + SQL reports | privacy, рост raw events |
| Web analytics | site registration, small tracker, SPA/pageview/custom events, reports | `webanalytics`, explicit origins, bounded collector | forged/noisy events, PII in URL |
| Monitoring | GET/HEAD schedules, timeout/status, history, status/uptime/p95/incidents | scheduler leases, bounded worker, safe client | SSRF/DNS rebinding, resource exhaustion |
| Dashboard | overview links/sites/monitors/events | read-only `dashboard` projections | смешение units и дорогие запросы |
| Frontend | landing, seven dashboard sections, responsive UX states | React feature structure + design system | шаблонный UI, fake production data |
| Operations | Compose, config, Caddy/HTTPS, CI, backup/health | staged backlog, production docs last | ложное предположение о DNS/TLS readiness |

Пользовательские сценарии A–E из SPEC имеют прямой путь: A — links/redirect; B — domains + host routing; C — sites/tracker/collector; D — monitoring; E — транзакционная связь link ↔ monitor.

## 2. Нефункциональные требования

### Security

- Tenant isolation — owner predicate в каждом private repository query и `404` для foreign IDs; отдельные integration tests двумя пользователями.
- Authentication — Argon2id, random server sessions, token hash at rest, secure host-only cookie, CSRF/Origin checks и rate limits.
- SSRF — pinned-IP dialer, full IP range checks, redirects/proxy/custom headers disabled и bounded resources.
- Input/data — DTO validation, parameterized pgx SQL, payload/URL/property limits, log redaction.
- Domain/TLS — TXT ownership обязательно; CNAME не доказательство; Caddy issuance только active allow-list.

### Reliability and correctness

- Мультизаписные изменения транзакционны; quota и click event не расходятся.
- Scheduler lease восстанавливается после crash; network I/O не удерживает DB transaction.
- HTTP server и worker имеют общий controlled shutdown.
- UTC storage, явные timezone buckets, sample count для uptime и approximate labels.
- Health endpoints разделяют liveness/readiness и не раскрывают internals.

### Performance and scalability

SPEC не задаёт SLO/объёмы, поэтому проект не обещает конкретный RPS. Hot paths получают составные индексы; event tables — time indexes и retention. Один VPS/DB является сознательной начальной границей. Перед partitioning/cache/queue нужны измерения (`EXPLAIN ANALYZE`, table size, redirect latency, ingest rate).

Предварительные engineering budgets, подлежащие утверждению и нагрузочной проверке позже: redirect backend p95 < 150 ms без учёта сети клиента; API p95 < 500 ms для стандартного 30-day report; tracker gzip < 10 KiB; collector payload <= 32 KiB; monitor concurrency default 10. Это цели, а не уже выполненные критерии.

### Privacy

- Полный IP не хранится и не передаётся GeoIP API; optional local GeoIP позже.
- Tracker не читает формы/content, не ставит cookie и очищает sensitive query.
- Приблизительные уникальные посетители — ограниченный HMAC, не «уникальный человек».
- Raw retention ограничен и конфигурируем; hard delete semantics сообщаются пользователю.

### Accessibility and UX

- WCAG AA contrast, keyboard/focus/dialog semantics, reduced motion, chart text alternatives.
- Loading/error/empty/success для каждого async screen; production без fixtures.
- Desktop/mobile layouts и корректная обработка длинных URL.

### Maintainability and testability

- Модульный монолит, Fiber/pgx только в adapters, explicit SQL, `context.Context`.
- OpenAPI синхронизирует Go contract и generated TS types.
- Unit, PostgreSQL integration, security, component и Playwright tests разбиты по issue; CI сообщает только фактически выполненные проверки.

## 3. Вне MVP / отложено

- email/Telegram alerts, custom secret headers и произвольные POST monitor requests;
- рекламная атрибуция, fingerprinting, cross-site identity;
- автоматизированное изменение DNS и production deployment;
- Redis/message broker/microservices/warehouse/partitioning без метрик;
- следование redirects монитором и произвольные target ports;
- сложная JSON assertion (поле оставлено как будущая возможность, не обещание API MVP).

## 4. Открытые продуктовые параметры

Открыты сроки действия сессий кабинета, политика паролей, квоты сущностей сверх утверждённого лимита поддомена, диапазоны отчётов и тексты публичных страниц.

## 5. Критерий готовности этапа 1

- существующий репозиторий и весь SPEC изучены;
- модули, зависимости и каталоги определены;
- схема БД, keys/indexes/relations и ERD подготовлены;
- Container/Component diagrams подготовлены;
- API surface и основные DTO определены;
- routing/redirect/worker/SSRF спроектированы;
- frontend design system и pages описаны;
- этапы 2–12 разбиты на issue-sized задачи с зависимостями и acceptance;
- противоречия/допущения перечислены; код/DNS/deployment не выполнялись.

