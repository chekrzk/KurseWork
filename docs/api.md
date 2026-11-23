# REST API и DTO

Архивирование: PATCH /links/{id} со status=archived. Окончательное удаление требует DELETE с confirm=true и подтверждения в интерфейсе. Collector принимает JSON одного события (до 32 KiB), OPTIONS разрешает только POST и Content-Type для точного origin активного сайта, Vary: Origin, credentials отключены. Для preflight site_id передаётся в query и сверяется с body при POST. Проверка Origin обязательна и на самом POST.

## 1. Соглашения

- Base path: `/api/v1`; JSON UTF-8; поля `snake_case`; timestamps RFC 3339 UTC.
- Cookie session: `saturnita_session`, host-only для `saturnita.ru`, `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`. Не задавать `Domain=.saturnita.ru`: redirect-hostам cookie не нужна.
- Все изменяющие приватные запросы требуют CSRF header `X-CSRF-Token` и same-origin check. Collector и redirect не используют пользовательскую сессию.
- IDs — UUID strings. Денежных/float метрик нет; проценты передаются как числа 0..100 с описанной точностью.
- `GET` списки: `limit` (default 25, max 100), opaque `cursor`; ответ содержит `next_cursor`.
- Analytics: `from`, `to`, `granularity`; максимум диапазона для raw endpoints — 180 дней.
- Проект OpenAPI 3.1 хранится в docs/openapi.json и является источником generated TypeScript types.

Единая ошибка:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "Проверьте введённые данные",
    "fields": {"slug": ["Slug уже занят"]},
    "request_id": "01J..."
  }
}
```

Коды: `400` malformed, `401` unauthenticated, `403` CSRF/forbidden action, `404` missing or foreign resource, `409` unique/state conflict, `422` semantic validation, `429` rate limit, `500` internal. Публичный redirect использует собственные HTML/HTTP ответы, не JSON.

## 2. Identity

| Метод и путь | Назначение | Основной ответ |
|---|---|---|
| `POST /auth/register` | регистрация и session | `201 UserDTO` |
| `POST /auth/login` | вход | `200 UserDTO` |
| `POST /auth/logout` | revoke текущей session | `204` |
| `GET /me` | профиль + CSRF bootstrap | `200 MeDTO` |
| `PATCH /me` | имя/timezone/locale/preferences | `200 UserDTO` |
| `PUT /me/password` | смена пароля | `204` |
| `GET /me/sessions` | активные сессии | page `SessionDTO` |
| `DELETE /me/sessions/{id}` | завершить сессию | `204` |

```ts
type RegisterRequest = { email: string; password: string; display_name: string };
type LoginRequest = { email: string; password: string };
type UserDTO = { id: string; email: string; display_name: string; timezone: string; locale: string; created_at: string };
type MeDTO = UserDTO & { preferences: Record<string, unknown>; csrf_token: string };
type ChangePasswordRequest = { current_password: string; new_password: string };
type SessionDTO = { id: string; current: boolean; user_agent: string | null; created_at: string; last_seen_at: string; expires_at: string };
```

Password policy и rate limits описываются в OpenAPI/UX; login не раскрывает существование email.

## 3. Links и link analytics

| Метод и путь | Назначение |
|---|---|
| `GET /links?query=&status=&domain_id=&cursor=` | список/поиск |
| `POST /links` | создать link, опционально monitor |
| `GET /links/{id}` | детали |
| `PATCH /links/{id}` | name, target, UTM, status, expiry, limit; slug immutable |
| `DELETE /links/{id}` | hard delete после UI confirmation |
| `GET /links/{id}/analytics` | summary + time series + breakdowns |
| `GET /links/{id}/clicks` | последние события, privacy-safe |
| `GET /link-analytics` | cross-link report |

```ts
type UTMDTO = { source?: string; medium?: string; campaign?: string; term?: string; content?: string };
type CreateLinkRequest = {
  name: string; target_url: string; slug?: string; domain_id: string;
  expires_at?: string | null; click_limit?: number | null; utm?: UTMDTO;
  monitor_target?: boolean;
};
type LinkDTO = {
  id: string; name: string; target_url: string; slug: string; domain_id: string;
  short_url: string; status: "active"|"inactive"|"archived";
  expires_at: string|null; click_limit: number|null; click_count: number;
  utm: UTMDTO; monitor_id: string|null; created_at: string; updated_at: string;
};
type TimePointDTO = { bucket: string; value: number };
type BreakdownDTO = { key: string; label: string; value: number; percentage: number };
type LinkAnalyticsDTO = {
  range: { from: string; to: string; granularity: "hour"|"day" };
  totals: { clicks: number; daily_unique_visitors_sum: number; bots: number };
  comparison: { clicks_change_percent: number|null };
  series: TimePointDTO[];
  devices: BreakdownDTO[]; sources: BreakdownDTO[]; campaigns: BreakdownDTO[]; countries: BreakdownDTO[];
};
```

Redirect endpoint не входит в `/api`: `GET|HEAD https://{registered-host}/{slug}`. Успех — `302 Location`; inactive/expired/missing — branded `404` или `410` без раскрытия причины; exhausted quota — `410`. `HEAD` не записывает click и возвращает тот же status/Location без body.

## 4. Domains

| Метод и путь | Назначение |
|---|---|
| `GET /domains` | доступные system + owned domains |
| `POST /domains/subdomains` | занять `{label}.saturnita.ru` |
| `POST /domains/custom` | создать pending external domain/challenge |
| `GET /domains/{id}` | статус и DNS instructions |
| `POST /domains/{id}/verify` | DNS TXT lookup, state transition |
| `POST /domains/{id}/activate` | проверить routing prerequisites и active |
| `POST /domains/{id}/verification/rotate` | новый challenge |
| `DELETE /domains/{id}` | удалить, если нет links |

```ts
type DomainDTO = {
  id: string; hostname: string; kind: "system"|"subdomain"|"custom";
  status: "pending"|"verified"|"active"|"error"|"disabled";
  verification?: { txt_name: string; txt_value: string; expires_at: string };
  dns?: { cname_name: string; cname_target: string };
  verified_at: string|null; created_at: string;
};
type CreateSubdomainRequest = { label: string };
type CreateCustomDomainRequest = { hostname: string };
```

Internal-only `GET /internal/caddy/allow-domain?domain=...` доступен только Caddy по private network/token и возвращает `200` лишь для active custom domain; публичным API не является.

## 5. Sites и collector

| Метод и путь | Назначение |
|---|---|
| `GET /sites`, `POST /sites` | list/create |
| `GET /sites/{id}`, `PATCH /sites/{id}`, `DELETE /sites/{id}` | CRUD |
| `PUT /sites/{id}/origins` | заменить explicit allow-list |
| `GET /sites/{id}/snippet` | tracker snippet |
| `GET /sites/{id}/analytics` | summary/series/breakdowns |
| `POST /collect?site_id={public_id}` | публичный сбор, `204` |

```ts
type CreateSiteRequest = { name: string; hostname: string; timezone: string; origins: string[] };
type SiteDTO = { id: string; public_id: string; name: string; hostname: string; timezone: string; status: "active"|"disabled"; origins: string[]; created_at: string };
type CollectEventRequest = {
  event_id: string; site_id: string; type: "pageview"|"custom"; name: string;
  occurred_at: string; path?: string; referrer?: string; utm?: Pick<UTMDTO,"source"|"medium"|"campaign">;
  properties?: Record<string, string|number|boolean|null>;
};
```

Один запрос содержит одно событие размером до 32 KiB. Сервер ограничивает отклонение occurred_at от времени приёма, проверяет плоские свойства и очищает путь повторно после tracker. Rate limit применяется по сайту и сетевому префиксу. Неизвестный и отключённый сайты обрабатываются одинаково.

## 6. Monitoring

| Метод и путь | Назначение |
|---|---|
| `GET /monitors`, `POST /monitors` | list/create |
| `GET /monitors/{id}`, `PATCH /monitors/{id}`, `DELETE /monitors/{id}` | CRUD |
| `POST /monitors/{id}/pause`, `POST /monitors/{id}/resume` | явные transitions |
| `POST /monitors/{id}/check` | поставить ближайшую проверку; не синхронный HTTP proxy |
| `GET /monitors/{id}/checks` | history page |
| `GET /monitors/{id}/analytics` | uptime/latency/series |
| `GET /monitors/{id}/incidents` | incidents page |

```ts
type CreateMonitorRequest = {
  name: string; url: string; kind: "website"|"api";
  method: "GET"|"HEAD"; interval_seconds: number; timeout_ms: number;
  expected_status: { min: number; max: number };
};
type MonitorDTO = {
  id: string; link_id: string|null; name: string; url: string;
  kind: "link_target"|"website"|"api"; method: "GET"|"HEAD";
  interval_seconds: number; timeout_ms: number; expected_status: { min: number; max: number };
  enabled: boolean; status: "operational"|"degraded"|"down"|"paused"|"unknown";
  last_checked_at: string|null; last_success_at: string|null; next_check_at: string|null;
};
type MonitorCheckDTO = { id: string; checked_at: string; http_status: number|null; latency_ms: number|null; success: boolean; error_type: string|null };
type MonitorAnalyticsDTO = { uptime_percent: number|null; sample_count: number; average_latency_ms: number|null; p95_latency_ms: number|null; series: Array<{ bucket: string; uptime_percent: number; p95_latency_ms: number|null }> };
type IncidentDTO = { id: string; started_at: string; ended_at: string|null; status: "open"|"resolved"; cause: string|null };
```

Link monitor создаётся/удаляется через link use case; прямой `POST /monitors` не принимает `link_id` или kind `link_target`, чтобы нельзя было связать чужую ссылку.

## 7. Dashboard и health

- `GET /dashboard/overview?from=&to=` → counts links/sites/monitors, click series, top links, recent privacy-safe events, monitor status summary и open incidents.
- `GET /healthz` → process alive, без DB dependency.
- `GET /readyz` → DB reachable и migrations compatible; worker health отображается отдельным полем, но временное отсутствие due jobs не означает failure.

Health не раскрывает DSN, версии пакетов, SQL errors или внутренние hostnames.

