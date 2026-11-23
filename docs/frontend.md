# Frontend и дизайн-система

Основное действие в меню ссылки — «Архивировать». Статистика остаётся доступной. Для «Удалить навсегда» диалог перечисляет клики, привязанный монитор и его историю. На графике метрика называется «Приблизительные уникальные посетители». Подпись для периода — «Сумма дневных значений (UTC). Посетитель может учитываться в нескольких днях».

## 1. Визуальное направление

Образ: premium analytics SaaS / developer tool с темой орбит и навигации, но без буквального «космоса повсюду». Светлый рабочий canvas для длительного чтения, тёмные public/hero surfaces и точечные violet/lime сигналы. Градиенты — только как декоративный акцент, не фон каждой карточки.

## 2. Design tokens

### Цвет

| Token | Light | Dark/hero | Назначение |
|---|---|---|---|
| `bg` | `#F6F5F2` | `#10131D` | canvas |
| `surface` | `#FFFFFF` | `#171B28` | cards/panels |
| `surface-muted` | `#EEEDE8` | `#1B2030` | secondary areas |
| `text` | `#171923` | `#F5F3EE` | primary text |
| `text-muted` | `#687083` | `#A6ADBD` | secondary text |
| `border` | `#DCDDE2` | `#30374A` | dividers |
| `primary` | `#7868EA` | `#8B7CF6` | actions/data series |
| `accent` | `#91C94D` | `#B9F272` | positive highlight |
| `success` | `#16875E` | `#42C98A` | operational |
| `warning` | `#B87516` | `#E5A84B` | degraded |
| `danger` | `#C34F5B` | `#E16B75` | down/error |
| `info` | `#3978C6` | `#73A9E8` | neutral info |

Контраст проверяется по WCAG AA; цвет никогда не является единственным носителем статуса. Chart palette: violet, lime, blue, amber, coral; соседние series отличаются также dash/marker.

### Типографика и геометрия

- Заголовки: Space Grotesk 600/700 с кириллицей.
- UI/body: Manrope 400/500/600 с кириллицей.
- URL, slug, latency: JetBrains Mono 400/500.
- Scale: 12, 14, 16, 18, 24, 32, 48/56 px; body 14/20 или 16/24.
- Spacing: 4 px base; основные шаги 8, 12, 16, 24, 32, 48, 64.
- Radius: 8 inputs, 12 cards, 16 feature panels; pill только для badges.
- Shadows: один subtle elevation; focus ring 2 px primary с offset.
- Motion: 120–180 ms UI, 300–500 ms hero; полностью отключаем nonessential motion при `prefers-reduced-motion`.

Шрифты self-hosted WOFF2; если конкретный font build не содержит кириллицу, используется проверенный subset или fallback `Inter, system-ui` без скрытого network fetch.

## 3. Компонентные уровни

`components/ui` оборачивает shadcn/Radix: Button, IconButton, Input, Select, Checkbox, Switch, Tabs, Dialog, Dropdown, Tooltip, Toast, Badge, Skeleton. Варианты задаются tokens, а не page-specific hex.

`components/data-display`: MetricCard, Trend, DataTable, EmptyState, StatusBadge, URLCell, CopyButton, DateRangePicker, Pagination, DefinitionList, EventFeed.

`components/charts`: ChartFrame, TimeSeriesChart, BreakdownBar, DonutChart, UptimeStrip, shared legend/tooltip/empty overlay. Recharts находится только внутри этого слоя.

`components/feedback`: PageError, InlineError, LoadingPanel, ConfirmDeleteDialog, FormField. Все async regions имеют loading/error/empty/success semantics и aria-live там, где нужно.

Feature-компоненты не импортируют raw API client напрямую: TanStack Query hooks живут рядом с feature, Zod schema синхронизируется с generated OpenAPI types.

## 4. Layout и навигация

Desktop: 248 px collapsible sidebar, 64 px topbar, content max 1440 px. Sidebar: Overview, Links, Analytics, Websites, Monitoring, Domains; Settings отделён снизу. Topbar показывает page context, date filter/actions и user menu.

Mobile: compact topbar + sheet navigation; primary action остаётся доступным, таблицы переключаются на semantic cards там, где horizontal scroll скрывает ключевые действия. Breakpoints ориентированы на содержимое, минимум touch target 44×44 px.

Маршруты:

```text
/                         landing
/login, /register         auth
/app                      overview
/app/links                links list/create
/app/links/:id            details + analytics
/app/analytics            cross-link analytics
/app/sites                websites
/app/sites/:id            web analytics
/app/monitors             monitoring list
/app/monitors/:id         checks/latency/incidents
/app/domains              domains + DNS instructions
/app/settings             profile/security/sessions
```

Protected layout сначала получает `/me`; `401` ведёт на login с safe return path. Никаких private data в localStorage; только неопасные UI preferences.

## 5. Структура страниц

### Landing

Тёмный editorial hero, интерактивная, но локальная демонстрация создания ссылки, один выразительный dashboard preview, три feature narratives, domains explainer, monitoring strip, CTA/footer. Орбитальная SVG-графика декоративна (`aria-hidden`), не блокирует LCP и не требует raster assets.

### Overview — эталон этапа 3

Верх: title/date range/quick-create. Затем четыре разнотипные metric cards с ясными units. Главная область: большой click trend (2/3) и monitor health (1/3). Ниже top links table и recent events. При отсутствии данных — onboarding checklist вместо нулевых выдуманных графиков. Этот экран утверждает систему до реализации остальных страниц.

### Links

Quick-create drawer/modal: target URL → auto slug/domain → optional advanced settings. Таблица: name/short URL/target/status/clicks/monitor/updated/actions. Long URLs ellipsis + tooltip/copy, но accessible full value. Filters отражаются в URL search params.

### Analytics

Обязательные range/granularity, summary с comparison, большой time-series, source/device breakdowns, performance table. Tooltip показывает точную дату/timezone и sample basis. Empty state предлагает создать/распространить link.

### Websites

List → create (domain, timezone, allowed origins) → integration snippet with copy/test instructions. Detail отделяет pageviews, pages, referrers, UTM и custom events; явно маркирует Приблизительные уникальные посетители.

### Monitoring

List rows с status icon+label, URL, interval, last check, latency. Detail: hero status, uptime/sample count, p95, uptime strip, latency chart, checks and incidents tabs. Ошибка показывает sanitized category, не тело ответа.

### Domains и Settings

Domains показывают lifecycle stepper и точные TXT/CNAME records с copy actions, verification errors и local-demo disclaimer. Settings: profile, password, sessions; destructive actions отделены.

## 6. Accessibility и quality gates

- semantic headings/landmarks, keyboard navigation, visible focus, dialogs restore focus;
- charts имеют текстовое summary/table alternative;
- tooltips не содержат единственную важную информацию;
- forms связывают label/error/description через ARIA;
- timezone и approximation подписаны;
- production build не содержит demo fixtures; demo data включаются только явным seed/env mode;
- component tests покрывают states, responsive smoke — Playwright на desktop/mobile.

