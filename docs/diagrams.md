# Диаграммы

Для обоих потоков событий visitor_hash хранит дневной псевдоним. Статистика принадлежит ссылке или сайту, результаты проверок — монитору. Сессии SESSIONS используются для входа в кабинет.

Диаграммы используют Mermaid и должны рендериться в GitHub. C4 показан обычными flowchart-блоками для совместимости с GitHub Mermaid.

## ERD

```mermaid
erDiagram
    USERS ||--o{ SESSIONS : has
    USERS o|--o{ DOMAINS : owns
    USERS ||--o{ LINKS : owns
    DOMAINS ||--o{ LINKS : hosts
    LINKS ||--o{ LINK_CLICKS : records
    USERS ||--o{ SITES : owns
    SITES ||--o{ SITE_ORIGINS : allows
    SITES ||--o{ WEB_EVENTS : records
    USERS ||--o{ MONITORS : owns
    LINKS o|--o| MONITORS : target_monitor
    MONITORS ||--o{ MONITOR_CHECKS : produces
    MONITORS ||--o{ INCIDENTS : opens

    USERS {
      uuid id PK
      varchar email UK
      text password_hash
      varchar timezone
      timestamptz created_at
    }
    SESSIONS {
      uuid id PK
      uuid user_id FK
      bytea token_hash UK
      timestamptz expires_at
      timestamptz revoked_at
    }
    DOMAINS {
      uuid id PK
      uuid user_id FK
      text kind
      varchar hostname UK
      text status
      text challenge
      timestamptz challenge_expires_at
    }
    LINKS {
      uuid id PK
      uuid user_id FK
      uuid domain_id FK
      varchar slug
      varchar target_url
      text status
      bigint click_count
      bigint click_limit
    }
    LINK_CLICKS {
      uuid id PK
      uuid link_id FK
      timestamptz occurred_at
      bytea visitor_hash
    }
    SITES {
      uuid id PK
      uuid user_id FK
      uuid public_id UK
      varchar hostname
      text status
    }
    SITE_ORIGINS {
      uuid site_id PK,FK
      varchar origin PK
    }
    WEB_EVENTS {
      bytea visitor_hash
      uuid id PK
      uuid site_id PK,FK
      timestamptz occurred_at
      timestamptz received_at
      text kind
      text name
      jsonb properties
    }
    MONITORS {
      uuid lease
      timestamptz lease_until
      uuid id PK
      uuid user_id FK
      uuid link_id FK
      varchar url
      int interval_seconds
      text status
      timestamptz next_check_at
    }
    MONITOR_CHECKS {
      timestamptz scheduled_at
      uuid id PK
      uuid monitor_id FK
      timestamptz checked_at
      int latency_ms
      boolean success
      text error_type
    }
    INCIDENTS {
      uuid id PK
      uuid monitor_id FK
      timestamptz started_at
      timestamptz ended_at
      text cause
    }
```

`slug` уникален не глобально, а составным ключом `(domain_id, slug)`. На диаграмме это сокращено из-за ограничений нотации.

## C4 Container

```mermaid
flowchart LR
  user["Person: пользователь"]
  visitor["Person: посетитель короткой ссылки"]
  site["External website + tracker"]
  target["External HTTP targets"]
  dns["DNS / public CA"]

  subgraph boundary["Saturnita Pulse"]
    caddy["Caddy\nreverse proxy, TLS, host routing"]
    spa["React SPA\nlanding + dashboard"]
    backend["Go modular monolith\nFiber API, redirects, collector, worker"]
    db[("PostgreSQL\ntransactional + event data")]
    tracker["tracker.js\nsmall dependency-free bundle"]
  end

  user -->|HTTPS UI/API| caddy
  visitor -->|HTTPS host/slug| caddy
  site -->|GET tracker + POST events| caddy
  caddy -->|static| spa
  caddy -->|/tracker.js| tracker
  caddy -->|/api, redirect hosts| backend
  backend -->|pgx| db
  backend -->|safe HTTP GET/HEAD| target
  caddy -. certificate validation .-> dns
```

## C4 Component — Go backend

```mermaid
flowchart TB
  caddy["Caddy"]
  db[("PostgreSQL")]
  internet["Public HTTP targets / DNS"]

  subgraph go["Container: Go modular monolith"]
    router["HTTP transport\nFiber routes + middleware"]
    identity["Identity component\nsessions, CSRF, profile"]
    domains["Domains component\nregistry + verification"]
    links["Links component\nCRUD + redirect resolver"]
    la["Link Analytics\nclick writer + reports"]
    wa["Web Analytics\nsites + collector + reports"]
    monitor["Monitoring component\nCRUD + state/incidents"]
    worker["Monitoring worker\nscheduler + bounded pool"]
    dashboard["Dashboard queries\noverview composition"]
    repos["PostgreSQL adapters\nexplicit SQL + tx"]
    netguard["Safe HTTP client\nDNS/IP/dial guard"]
    platform["Platform\nconfig, logs, clock, IDs, limits"]
  end

  caddy --> router
  router --> identity
  router --> domains
  router --> links
  router --> la
  router --> wa
  router --> monitor
  router --> dashboard
  links --> la
  links -. ensure monitor .-> monitor
  worker --> monitor
  worker --> netguard
  netguard --> internet
  identity --> repos
  domains --> repos
  links --> repos
  la --> repos
  wa --> repos
  monitor --> repos
  dashboard --> repos
  repos --> db
  platform -. shared technical services .-> router
  platform -. shared technical services .-> worker
```

