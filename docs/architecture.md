# Архитектура и анализ требований

## 1. Контекст и движущие требования

Saturnita Pulse объединяет четыре нагруженных по-разному сценария: управление ссылками, быстрый публичный redirect со сбором кликов, приём web-analytics событий и периодические исходящие HTTP-проверки. Общие требования — строгая изоляция пользователей, privacy-friendly аналитика, безопасная работа с произвольными URL, качественный responsive UI и эксплуатация на одном VPS без лишней инфраструктуры.

Нефункциональные приоритеты в порядке важности:

1. безопасность и tenant isolation;
2. корректность redirect и мониторинга;
3. предсказуемая эксплуатация и восстановление;
4. достаточная производительность на одном экземпляре;
5. ясность кода и тестируемость;
6. доступность и визуальная целостность frontend.

Redis, Kafka, Kubernetes, message broker, отдельный analytics warehouse и микросервисы в MVP не нужны. PostgreSQL покрывает транзакции, координацию worker и аналитические выборки начального масштаба.

## 2. Архитектурный стиль

Backend — модульный монолит. Каждый бизнес-модуль содержит модель, use cases, repository-интерфейсы и HTTP-адаптеры, но не становится отдельным сервисом. Общие технические средства находятся в `platform`, а композиция зависимостей — только в `cmd/api`.

Направление зависимостей:

```text
transport/http -> application/use cases -> domain + repository ports
                                      <- postgres adapters
worker --------> monitoring use cases -> repository + safe HTTP client
```

Fiber существует только на транспортной границе. Доменные и прикладные пакеты принимают `context.Context`, обычные Go-типы и интерфейсы. SQL находится только в postgres-адаптерах. Для первой версии предпочтительны явные запросы pgx без sqlc: схема ещё будет уточняться, а объём запросов управляем. Решение можно пересмотреть после стабилизации OpenAPI и миграций.

## 3. Границы модулей

| Модуль | Ответственность | Не отвечает за |
|---|---|---|
| `identity` | регистрация, профиль, пароль, сессии, CSRF, ownership context | ссылки и аналитику |
| `domains` | системные/персональные/внешние домены, lifecycle, TXT verification, hostname lookup | TLS-выдачу как таковую |
| `links` | CRUD, slug, URL/UTM, quota, redirect resolution, архивирование | аналитические агрегации |
| `linkanalytics` | запись кликов, периодные агрегаты, Приблизительные уникальные посетители | web tracker события |
| `webanalytics` | сайты, origins, tracker ingestion, pageview/custom events, отчёты | link clicks |
| `monitoring` | monitors, scheduler, safe HTTP checks, checks, status machine, incidents | внешние notification channels |
| `dashboard` | read-only композиция сводных метрик | владение исходными данными |
| `platform` | config, pgx pool/transactions, clock, IDs, logging, middleware, rate limits, safe networking | бизнес-правила |

Модули связываются через небольшие интерфейсы/use cases. Например, `links` при включении мониторинга вызывает application facade `monitoring.EnsureForLink`, но не пишет напрямую в таблицу monitors. `dashboard` выполняет специализированные read queries, не копируя доменную логику.

## 4. Предлагаемая структура каталогов

```text
backend/
  cmd/api/main.go                  # composition root, server + worker lifecycle
  internal/
    identity/{domain,app,repo,http}/
    domains/{domain,app,repo,http}/
    links/{domain,app,repo,http}/
    linkanalytics/{app,repo,http}/
    webanalytics/{domain,app,repo,http}/
    monitoring/{domain,app,repo,http,worker}/
    dashboard/{app,repo,http}/
    platform/
      config/ db/ tx/ httpserver/ middleware/ logging/ netguard/ clock/
  migrations/
  tests/integration/
frontend/
  src/
    app/                          # router, providers, layouts
    pages/                        # route-level composition
    features/{auth,links,domains,link-analytics,sites,monitoring,dashboard}/
    components/{ui,charts,data-display,feedback}/
    lib/{api,validation,formatting}/
    styles/
    types/generated/              # позднее из OpenAPI
tracker/
  src/tracker.ts
openapi/
docs/
deploy/                           # только на production-readiness этапе
```

Внутри модуля допускаются плоские пакеты, если файлов мало. Не следует одновременно создавать общие `domain/service/repository` верхнего уровня и такие же слои внутри каждого feature: это породит дублирование.

## 5. Основные сценарии и транзакционные границы

### Регистрация и вход

Email нормализуется, пароль хешируется Argon2id, пользователь создаётся транзакционно. На login создаётся случайный 256-bit session token; клиент получает исходное значение только в cookie, БД хранит SHA-256 hash. Смена пароля инвалидирует остальные сессии. Изменяющие cookie-auth запросы требуют CSRF token (double-submit либо synchronizer token, выбранный при реализации) и проверки `Origin`.

### Создание ссылки

Use case проверяет ownership активного домена, нормализует URL/slug, запрещает внутренний redirect cycle, затем создаёт link. Уникальность `(domain_id, slug)` гарантирует БД. При запросе мониторинга в той же транзакции создаётся связанный monitor через transaction-aware facade.

### Redirect

Hostname и slug нормализуются, затем одной выборкой находится активная ссылка. Проверка quota выполняется атомарным `UPDATE ... WHERE click_count < click_limit RETURNING`; событие клика записывается в той же короткой транзакции. После commit возвращается `302`. Входящие query-параметры посетителя не добавляются автоматически; query исходного URL и явно настроенные UTM сохраняются.

### Подключение внешнего домена

Создаётся pending domain и одноразовый TXT challenge. Проверка выполняется backend по явному действию пользователя; CNAME подтверждает маршрутизацию, но только TXT подтверждает владение. После `verified` пользователь активирует домен, а Caddy получает разрешение на сертификат только через внутренний allow endpoint для `active` hostname.

### Сбор web events

Публичный `site_id` определяет сайт, Origin сверяется с allow-list, payload жёстко ограничивается. Collector не создаёт пользователей и не принимает PII. Событие записывается append-only; агрегаты считаются SQL-запросами. `Origin` и `site_id` — антиошибочные сигналы, не секрет и не доказательство подлинности.

## 6. Сквозные правила

- Все timestamps — `timestamptz` в UTC; пользовательский timezone применяется только при агрегации/выводе.
- Идентификаторы API — UUID. Публичный `site_id` — отдельный непоследовательный UUID.
- Ownership проверяется внутри SQL (`WHERE id=$1 AND user_id=$2`), а не отдельным check-then-act.
- Публичные ответы не различают «не найдено» и «чужой объект»: `404`.
- Ошибки логируются с request ID, но без password, cookie, CSRF, полного URL query с PII и полного IP.
- Все списки имеют cursor pagination; analytics имеют обязательный bounded range.
- Write use cases принимают idempotency только там, где повтор опасен; для MVP это collector `event_id` и, при необходимости, создание link из UI.
- Graceful shutdown сначала прекращает приём трафика/новых jobs, ждёт активные handlers/checks с deadline, затем закрывает pool.

## 7. Масштабирование и retention

Начальная цель — один backend instance и одна PostgreSQL. Индексы рассчитаны на range scans по владельцу и времени. При росте объёма первыми действиями будут retention purge батчами и месячное partitioning для `link_clicks`, `web_events`, `monitor_checks`; заранее создавать партиции в MVP не требуется.

Предлагаемый retention по умолчанию: raw link clicks и web events — 180 дней, monitor checks — 90 дней, incidents — до удаления monitor/user. Значения конфигурируемы и должны быть явно отражены в UI/политике. Purge выполняется небольшими SQL-батчами тем же maintenance loop, без отдельной системы очередей.

## 8. Противоречия, неопределённости и принятые решения

1. **«Самостоятельный worker» и «в том же приложении».** Для MVP worker — отдельный компонент/goroutine pool в процессе `cmd/api`, с независимым lifecycle. Отдельный binary/container не создаётся. Repository lease позволяет вынести его позже.
2. **`{username}.saturnita.ru` против свободно вводимого поддомена.** Поддомен трактуется как отдельный handle, а не автоматически как имя профиля. У пользователя может быть несколько доменов, но в MVP действует ограничение на один персональный поддомен.
3. **Удаление ссылок против сохранения аналитики.** «Архивировать» — soft state. Подтверждённое удаление — hard delete ссылки и её clicks/связанного monitor каскадом; UI обязан предупредить о потере статистики. Это проще и честнее требований MVP.
4. **UTM и запрет автоматического добавления query.** Сохраняется query целевого URL; настроенные в link UTM детерминированно добавляются/заменяют одноимённые UTM. Query входящего короткого URL игнорируется, пока не появится отдельная allow-list feature.
5. **Click limit и одновременные переходы.** Счётчик `click_count` денормализован в `links` и изменяется атомарно в транзакции вместе с кликом; иначе лимит нарушается при конкуренции.
6. **Единичный сбой и статус Down.** `Down` открывается после двух последовательных неуспехов; первый даёт `Degraded`. Успех закрывает инцидент и возвращает `Operational`. Порог позже станет настройкой.
7. **Redirect в monitor.** Для безопасного MVP redirects отключены. Это строже формулировки «перепроверять или запретить» и устраняет основной обход SSRF. Позже допустим максимум 3 hop с полной повторной проверкой.
8. **Приблизительные уникальные посетители.** Для каждого события сервер берёт UTC-день приёма. Дневной ключ = HMAC-SHA256(master, кодированные версия + день + область). Область — владелец для кликов и отдельный сайт для web events. Дайджест = HMAC-SHA256(дневной ключ, канонический IP из доверенного источника + нормализованный User-Agent). Поля кодируются с длинами, IPv4-mapped адрес приводится к IPv4, UA обрезается до 512 байт, пробелы нормализуются. В таблице остаются только 32 байта псевдонима. Исходные признаки, ключи и полные IP исключены из аналитики и логов. Master secret поступает из окружения. Это псевдонимизация. Для диапазона [from,to) считаем DISTINCT по каждому UTC-дню среди отобранных событий и суммируем дневные значения. Один человек в разные дни может учитываться несколько раз. Для web events границы этой метрики определяет received_at. Посетительские сессии не создаются.
9. **GeoIP.** Схема допускает `country_code`, но GeoIP-база не входит в начальный bootstrap и поле остаётся `NULL`; сетевые запросы третьим лицам не выполняются.
10. **API host.** Выбран same-origin `saturnita.ru/api/v1`, чтобы упростить cookie/CSRF/CORS. `api.saturnita.ru` зарезервирован, но не нужен для MVP.
11. **External TLS.** Проектируется Caddy On-Demand TLS с защищённым ask endpoint, разрешающим только active domains. Реальная настройка зависит от VPS/DNS и остаётся за пределами текущего этапа.
12. **Web-event authenticity.** Публичный tracker невозможно сделать доверенным только site ID/origin. MVP ограничивает ущерб validation/rate limits; метрики следует считать observational, а не финансово-аудиторскими.
13. **Порты monitor URL.** Безопасный MVP разрешает только 80/443. Поддержка других публичных портов вводится отдельной allow-list после threat review; это сознательно уже формулировки «произвольный HTTP endpoint».

## 9. Утверждённые решения

- hard delete со связанной аналитикой после явного подтверждения;
- retention 180/180/90 дней;
- один персональный поддомен на пользователя как продуктовый лимит MVP;
- monitor redirects disabled и Down после двух последовательных ошибок;
- same-origin API и отсутствие отдельного `api.saturnita.ru` в MVP.
- monitor только на портах 80/443 в MVP.

