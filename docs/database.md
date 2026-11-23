# PostgreSQL-схема

Физические ограничения и индексы приведены в [schema.sql](schema.sql). Частичный уникальный индекс domains(user_id) для kind=subdomain обеспечивает один персональный домен при конкурентной регистрации. В monitor_checks хранится scheduled_at. Очистка партиями затрагивает только события (clicks и web events 180 дней, checks 90 дней).

## 1. Общие соглашения

- PostgreSQL 16+; схема `public`, изменения только через `golang-migrate`.
- Primary keys: `uuid`, генерируются приложением. Время: `timestamptz`, UTC.
- Email/hostname/slug сравниваются в нормализованном lowercase-виде. Чтобы не требовать расширение `citext`, используются unique indexes по `lower(...)` и application normalization.
- Enum-подобные значения — `text` + `CHECK`: их проще расширять миграциями, чем PostgreSQL enum.
- На всех tenant-таблицах есть `user_id`; repository всегда включает его в приватные запросы.
- `updated_at` изменяет приложение явным SQL, database trigger не нужен.
- `jsonb` допустим только для ограниченных event properties и user preferences; основные фильтруемые поля нормализованы.

Ниже приведена логическая схема. Точные имена constraints фиксируются при создании миграций.

## 2. Таблицы

### `users`

| Поле | Тип | Ограничения |
|---|---|---|
| `id` | uuid | PK |
| `email` | varchar(320) | NOT NULL |
| `password_hash` | text | NOT NULL |
| `display_name` | varchar(120) | NOT NULL |
| `timezone` | varchar(64) | NOT NULL DEFAULT 'UTC' |
| `locale` | varchar(10) | NOT NULL DEFAULT 'ru' |
| `preferences` | jsonb | NOT NULL DEFAULT '{}'::jsonb, object only |
| `created_at`, `updated_at` | timestamptz | NOT NULL |

Индексы: `UNIQUE (lower(email))`.

### `sessions`

| Поле | Тип | Ограничения |
|---|---|---|
| `id` | uuid | PK |
| `user_id` | uuid | FK users ON DELETE CASCADE |
| `token_hash` | bytea | NOT NULL, UNIQUE |
| `csrf_secret_hash` | bytea | NULL/NOT NULL по выбранной CSRF-схеме |
| `user_agent` | varchar(512) | NULL, диагностическое поле |
| `created_at`, `last_seen_at`, `expires_at` | timestamptz | NOT NULL |
| `revoked_at` | timestamptz | NULL |

Индексы: `(user_id, created_at DESC)`, partial `(expires_at) WHERE revoked_at IS NULL`. Raw token не хранится.

### `domains`

| Поле | Тип | Ограничения |
|---|---|---|
| `id` | uuid | PK |
| `user_id` | uuid | FK users ON DELETE CASCADE, NULL только для system domain |
| `kind` | text | CHECK `system`, `subdomain`, `custom` |
| `hostname` | varchar(253) | NOT NULL |
| `status` | text | CHECK `pending`, `verified`, `active`, `error`, `disabled` |
| `verification_error` | varchar(500) | NULL |
| `verified_at`, `activated_at` | timestamptz | NULL |
| `created_at`, `updated_at` | timestamptz | NOT NULL |

Ограничения/индексы: `CHECK (hostname = lower(hostname))`; `UNIQUE (lower(hostname))`; `(user_id, status, created_at DESC)`; `CHECK ((kind='system') = (user_id IS NULL))`. Seed будущей миграции создаёт `go.saturnita.ru` как system/active. `saturnita.ru`, `www`, `api`, `admin`, `mail` не создаются пользовательскими доменами и проверяются application allow/deny rules.

### `domain_verifications`

| Поле | Тип | Ограничения |
|---|---|---|
| `id` | uuid | PK |
| `domain_id` | uuid | FK domains ON DELETE CASCADE |
| `token` | varchar(128) | NOT NULL, random public challenge |
| `txt_name` | varchar(253) | NOT NULL |
| `created_at`, `expires_at` | timestamptz | NOT NULL |
| `checked_at`, `succeeded_at` | timestamptz | NULL |

Индексы: unique partial `(domain_id) WHERE succeeded_at IS NULL`; `(expires_at)`. Старый challenge удаляется/закрывается при ротации. TXT token не является credential, но исключается из обычных логов.

### `links`

| Поле | Тип | Ограничения |
|---|---|---|
| `id` | uuid | PK |
| `user_id` | uuid | FK users ON DELETE CASCADE |
| `domain_id` | uuid | FK domains ON DELETE RESTRICT |
| `name` | varchar(160) | NOT NULL |
| `slug` | varchar(100) | NOT NULL |
| `target_url` | varchar(2048) | NOT NULL |
| `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, `utm_content` | varchar(255) | NULL |
| `status` | text | CHECK `active`, `inactive`, `archived` |
| `expires_at` | timestamptz | NULL |
| `click_limit` | bigint | NULL, CHECK > 0 |
| `click_count` | bigint | NOT NULL DEFAULT 0, CHECK >= 0 |
| `created_at`, `updated_at` | timestamptz | NOT NULL |

Ограничения: `CHECK (slug = lower(slug))`; `UNIQUE (domain_id, slug)`; `CHECK (click_limit IS NULL OR click_count <= click_limit)`. Индексы: `(user_id, status, created_at DESC)`, `(user_id, updated_at DESC)`, trigram/full-text не добавляется в MVP — поиск выполняется по ограниченному tenant dataset через `ILIKE`, индекс добавляется только по измерениям.

Владение доменом проверяется use case: system domain общедоступен, остальные должны иметь тот же `user_id` и `active` status.

### `link_clicks`

| Поле | Тип | Ограничения |
|---|---|---|
| `id` | uuid | PK |
| `link_id` | uuid | FK links ON DELETE CASCADE |
| `user_id` | uuid | FK users ON DELETE CASCADE, денормализовано для быстрых tenant ranges |
| `clicked_at` | timestamptz | NOT NULL |
| `referer_host` | varchar(253) | NULL |
| `utm_source`, `utm_medium`, `utm_campaign` | varchar(255) | NULL |
| `device`, `browser`, `os` | varchar(64) | NULL |
| `country_code` | char(2) | NULL |
| `is_bot` | boolean | NOT NULL DEFAULT false |
| `visitor_day_hash` | bytea | NULL, HMAC-SHA256, 32 байта |

Индексы: `(link_id, clicked_at DESC)`, `(user_id, clicked_at DESC)`, `(user_id, utm_campaign, clicked_at DESC)`, `(link_id, visitor_day_hash, clicked_at)` при доказанной необходимости unique queries. Referential consistency `link.user_id == click.user_id` обеспечивается repository транзакцией; при желании позднее — composite FK.

### `sites`

| Поле | Тип | Ограничения |
|---|---|---|
| `id` | uuid | PK |
| `user_id` | uuid | FK users ON DELETE CASCADE |
| `public_id` | uuid | NOT NULL, UNIQUE |
| `name` | varchar(160) | NOT NULL |
| `hostname` | varchar(253) | NOT NULL |
| `timezone` | varchar(64) | NOT NULL DEFAULT 'UTC' |
| `status` | text | CHECK `active`, `disabled` |
| `created_at`, `updated_at` | timestamptz | NOT NULL |

Индексы: `(user_id, created_at DESC)`, `UNIQUE (user_id, lower(hostname))`.

### `site_origins`

| Поле | Тип | Ограничения |
|---|---|---|
| `site_id` | uuid | FK sites ON DELETE CASCADE |
| `origin` | varchar(512) | NOT NULL, canonical scheme + host + optional port |
| `created_at` | timestamptz | NOT NULL |

PK `(site_id, origin)`. Wildcard origins в MVP запрещены, кроме явно документированного local-development режима.

### `web_events`

| Поле | Тип | Ограничения |
|---|---|---|
| `id` | uuid | PK; может совпадать с client event id для дедупликации |
| `site_id` | uuid | FK sites ON DELETE CASCADE |
| `user_id` | uuid | FK users ON DELETE CASCADE, денормализовано |
| `occurred_at`, `received_at` | timestamptz | NOT NULL |
| `event_type` | text | CHECK `pageview`, `custom` |
| `event_name` | varchar(80) | NOT NULL |
| `path` | varchar(2048) | NULL, очищенный path без sensitive query |
| `referrer_host` | varchar(253) | NULL |
| `utm_source`, `utm_medium`, `utm_campaign` | varchar(255) | NULL |
| `device`, `browser`, `os` | varchar(64) | NULL |
| `country_code` | char(2) | NULL |
| `visitor_day_hash` | bytea | NULL |
| `properties` | jsonb | NOT NULL DEFAULT '{}', object, max encoded size checked in app |

Индексы: `(site_id, occurred_at DESC)`, `(user_id, occurred_at DESC)`, `(site_id, event_name, occurred_at DESC)`, `(site_id, path, occurred_at DESC)`. Не создавать общий GIN до появления реального properties query use case.

### `monitors`

| Поле | Тип | Ограничения |
|---|---|---|
| `id` | uuid | PK |
| `user_id` | uuid | FK users ON DELETE CASCADE |
| `link_id` | uuid | FK links ON DELETE CASCADE, NULL |
| `name` | varchar(160) | NOT NULL |
| `url` | varchar(2048) | NOT NULL |
| `kind` | text | CHECK `link_target`, `website`, `api` |
| `method` | text | CHECK `GET`, `HEAD` |
| `interval_seconds` | integer | CHECK 300..86400 |
| `timeout_ms` | integer | CHECK 500..30000 и < interval |
| `expected_status_min`, `expected_status_max` | smallint | CHECK 100..599 и min <= max |
| `enabled` | boolean | NOT NULL |
| `status` | text | CHECK `operational`, `degraded`, `down`, `paused`, `unknown` |
| `consecutive_failures` | integer | NOT NULL DEFAULT 0 |
| `last_checked_at`, `last_success_at`, `next_check_at` | timestamptz | NULL |
| `lease_until` | timestamptz | NULL |
| `lease_owner` | uuid | NULL |
| `created_at`, `updated_at` | timestamptz | NOT NULL |

Индексы: `(user_id, created_at DESC)`, scheduler partial `(next_check_at) WHERE enabled`, `(lease_until) WHERE enabled`, unique partial `(link_id) WHERE link_id IS NOT NULL`. `link_target` требует link_id, остальные — NULL; это фиксируется CHECK. JSON-field assertion оставлена вне MVP.

### `monitor_checks`

| Поле | Тип | Ограничения |
|---|---|---|
| `id` | uuid | PK |
| `monitor_id` | uuid | FK monitors ON DELETE CASCADE |
| `user_id` | uuid | FK users ON DELETE CASCADE, денормализовано |
| `checked_at` | timestamptz | NOT NULL |
| `http_status` | smallint | NULL, CHECK 100..599 |
| `latency_ms` | integer | NULL, CHECK >= 0 |
| `success` | boolean | NOT NULL |
| `error_type` | text | NULL; allow-list (`dns`, `connect`, `tls`, `timeout`, `http_status`, `response_limit`, `blocked_target`, `other`) |
| `error_message` | varchar(500) | NULL, sanitized |
| `resolved_ip` | inet | NULL; только публичный dial IP для аудита SSRF |

Индексы: `(monitor_id, checked_at DESC)`, `(user_id, checked_at DESC)`, partial `(monitor_id, checked_at DESC) WHERE NOT success`. Response body не хранится.

### `incidents`

| Поле | Тип | Ограничения |
|---|---|---|
| `id` | uuid | PK |
| `monitor_id` | uuid | FK monitors ON DELETE CASCADE |
| `user_id` | uuid | FK users ON DELETE CASCADE |
| `started_at` | timestamptz | NOT NULL |
| `ended_at` | timestamptz | NULL |
| `status` | text | CHECK `open`, `resolved` |
| `cause` | text | NULL |
| `first_check_id`, `last_check_id` | uuid | FK monitor_checks ON DELETE SET NULL |
| `created_at`, `updated_at` | timestamptz | NOT NULL |

Индексы: `(monitor_id, started_at DESC)`, `(user_id, status, started_at DESC)`, unique partial `(monitor_id) WHERE status='open'`.

Внутренние dashboard alerts выводятся из open incidents и domain errors; отдельная `notifications` таблица до появления dismiss/read semantics не нужна.

## 3. Критические операции

### Атомарный redirect с quota

Транзакция блокирует/обновляет подходящую active link строку условным `UPDATE`, увеличивает `click_count`, вставляет `link_clicks`, commit. Для unlimited link счётчик также увеличивается. Если условие не вернуло строку, service различает expired/inactive/limit с одной дополнительной безопасной выборкой. Это гарантирует отсутствие превышения quota.

### Выдача monitor jobs

Каждая выдача due-задания через FOR UPDATE SKIP LOCKED получает новый lease_owner (UUID попытки) и lease_until. Сетевой запрос выполняется после commit. Завершение блокирует monitor и проверяет актуальность аренды и scheduled_at = next_check_at. Только действительный владелец одной транзакцией добавляет check, меняет состояние/инцидент, сдвигает next_check_at и снимает аренду. UNIQUE(monitor_id,scheduled_at) запрещает повторный результат. При остановке процесса после отправки HTTP возможен повтор сетевой попытки после истечения аренды. Pause или изменение конфигурации инвалидирует аренду.

## 4. Retention и удаление

- Периодический purge удаляет raw rows по indexed timestamp маленькими батчами.
- Удаление user каскадно удаляет все tenant data; system domains защищены отсутствием owner и отдельными административными миграциями.
- Удаление custom domain с существующими links блокируется `RESTRICT`; сначала ссылки удаляются.
- Backup/restore и production retention документируются на этапе 12.

