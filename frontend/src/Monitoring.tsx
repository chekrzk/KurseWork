import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, type User } from "./api";
import type { components } from "./product-generated";
import { Action, Feedback, useData, type Page } from "./product";
import { Form, Field } from "./ui";
type Monitor = components["schemas"]["Monitor"];
type Check = components["schemas"]["MonitorCheck"];
type Incident = components["schemas"]["Incident"];
type Stats = components["schemas"]["MonitorAnalytics"];
const labels = {
  unknown: "Ожидает проверки",
  operational: "Доступен",
  degraded: "Первая ошибка",
  down: "Недоступен",
  paused: "На паузе",
};
const when = (s: string | null) => (s ? new Date(s).toLocaleString("ru") : "—");
function Badge({ monitor }: { monitor: Monitor }) {
  return (
    <span className={"status-badge " + monitor.status}>
      {labels[monitor.status]}
    </span>
  );
}
function NumberField({
  name,
  label,
  min,
  max,
  value,
}: {
  name: string;
  label: string;
  min: number;
  max: number;
  value: number;
}) {
  return (
    <label>
      {label}
      <input
        type="number"
        required
        name={name}
        min={min}
        max={max}
        defaultValue={value}
      />
    </label>
  );
}
function Editor({
  user,
  monitor,
  saved,
}: {
  user: User;
  monitor?: Monitor;
  saved: () => void;
}) {
  return (
    <Form
      title={monitor ? "Настройки проверки" : "Новый HTTP-монитор"}
      label="Сохранить монитор"
      submit={async (d) => {
        const payload = {
          name: d.get("name"),
          ...(!monitor?.link_id
            ? { url: d.get("url"), kind: d.get("kind") }
            : {}),
          method: d.get("method"),
          interval_seconds: Number(d.get("interval")),
          timeout_ms: Number(d.get("timeout")),
          expected_status: {
            min: Number(d.get("min")),
            max: Number(d.get("max")),
          },
        };
        await api(
          monitor ? "/monitors/" + monitor.id : "/monitors",
          monitor ? "PATCH" : "POST",
          payload,
          user.csrf_token,
        );
        saved();
      }}
    >
      <Field label="Название" name="name" initial={monitor?.name} max={160} />
      {monitor?.link_id ? (
        <p>Целевой URL задаётся в настройках короткой ссылки.</p>
      ) : (
        <>
          <Field
            name="url"
            label="URL проверки"
            type="url"
            initial={monitor?.url}
          />
          <label>
            Тип
            <select name="kind" defaultValue={monitor?.kind ?? "website"}>
              <option value="website">Сайт</option>
              <option value="api">API</option>
            </select>
          </label>
        </>
      )}
      <div className="filters">
        <label>
          Метод
          <select name="method" defaultValue={monitor?.method ?? "HEAD"}>
            <option>HEAD</option>
            <option>GET</option>
          </select>
        </label>
        <NumberField
          name="interval"
          label="Интервал, с"
          min={300}
          max={86400}
          value={monitor?.interval_seconds ?? 300}
        />
        <NumberField
          name="timeout"
          label="Таймаут, мс"
          min={500}
          max={30000}
          value={monitor?.timeout_ms ?? 5000}
        />
      </div>
      <div className="filters">
        <NumberField
          name="min"
          label="HTTP-статус от"
          min={100}
          max={599}
          value={monitor?.expected_status.min ?? 200}
        />
        <NumberField
          name="max"
          label="HTTP-статус до"
          min={100}
          max={599}
          value={monitor?.expected_status.max ?? 299}
        />
      </div>
    </Form>
  );
}
export function Monitoring({ user }: { user: User }) {
  const list = useData<Page<Monitor>>("/monitors");
  const [form, setForm] = useState(false);
  return (
    <>
      <header>
        <h1>Мониторинг</h1>
        <button onClick={() => setForm(!form)}>
          {form ? "Закрыть форму" : "Добавить монитор"}
        </button>
      </header>
      {form && (
        <Editor
          user={user}
          saved={() => {
            setForm(false);
            list.reload();
          }}
        />
      )}
      <Feedback error={list.error} loading={!list.data} />
      {list.data && !list.data.items.length && (
        <section className="card">
          <h2>Проверок пока нет</h2>
          <p>
            Добавьте публичный сайт или API для периодической проверки
            доступности.
          </p>
        </section>
      )}
      {list.data?.items.map((m) => (
        <article key={m.id} className="card resource">
          <Badge monitor={m} />
          <h2>
            <Link to={"/app/monitors/" + m.id}>{m.name}</Link>
          </h2>
          <p>
            {m.method} · {m.url}
          </p>
          <p>
            Последняя проверка {when(m.last_checked_at)} · каждые{" "}
            {m.interval_seconds} с
          </p>
        </article>
      ))}
    </>
  );
}
export function MonitorDetail({ user }: { user: User }) {
  const { id } = useParams();
  const base = "/monitors/" + id;
  const [days, setDays] = useState(7),
    [stamp, refresh] = useState(() => Date.now()),
    [edit, setEdit] = useState(false);
  useEffect(() => {
    if (edit) return;
    const timer = setInterval(() => refresh(Date.now()), 15000);
    return () => clearInterval(timer);
  }, [edit]);
  const range =
    "?from=" +
    encodeURIComponent(new Date(stamp - days * 86400000).toISOString()) +
    "&to=" +
    encodeURIComponent(new Date(stamp).toISOString());
  const m = useData<Monitor>(base + "?at=" + stamp),
    checks = useData<Page<Check>>(base + "/checks" + range),
    stats = useData<Stats>(base + "/analytics" + range),
    inc = useData<Page<Incident>>(base + "/incidents" + range);
  const [deleted, setDeleted] = useState(false);
  if (deleted) return <Link to="/app/monitors">Монитор удалён. К списку</Link>;
  const reload = () => refresh(Date.now());
  const action = async (a: string) => {
    await api(base + "/" + a, "POST", undefined, user.csrf_token);
    reload();
  };
  return (
    <>
      <Link to="/app/monitors">Все мониторы</Link>
      <Feedback
        error={m.error || stats.error || checks.error || inc.error}
        loading={!m.data}
      />
      {m.data && (
        <>
          <header>
            <div>
              <h1>{m.data.name}</h1>
              <p>{m.data.url}</p>
            </div>
            <Badge monitor={m.data} />
          </header>
          <div className="actions">
            <Action run={() => action(m.data!.enabled ? "pause" : "resume")}>
              {m.data.enabled ? "На паузу" : "Возобновить"}
            </Action>
            {m.data.enabled && (
              <Action run={() => action("check")}>Проверить сейчас</Action>
            )}
            <button onClick={() => setEdit(!edit)}>Настройки</button>
            <Action
              run={async () => {
                if (confirm("Удалить монитор, проверки и инциденты?")) {
                  await api(base, "DELETE", undefined, user.csrf_token);
                  setDeleted(true);
                }
              }}
            >
              Удалить
            </Action>
          </div>
          {edit && (
            <Editor
              monitor={m.data}
              user={user}
              saved={() => {
                setEdit(false);
                reload();
              }}
            />
          )}
          <p>
            Последняя успешная проверка {when(m.data.last_success_at)} ·
            Следующая {when(m.data.next_check_at)}
          </p>
        </>
      )}
      <div className="filters">
        <label>
          Период
          <select
            value={days}
            onChange={(e) => setDays(Number(e.target.value))}
          >
            <option value={1}>24 часа</option>
            <option value={7}>7 дней</option>
            <option value={30}>30 дней</option>
            <option value={90}>90 дней</option>
          </select>
        </label>
      </div>
      {stats.data && (
        <>
          <section className="metrics">
            {[
              ["Доступность", stats.data.uptime_percent?.toFixed(1) + " %"],
              [
                "Средняя задержка",
                stats.data.average_latency_ms?.toFixed(0) + " мс",
              ],
              ["p95 задержки", stats.data.p95_latency_ms?.toFixed(0) + " мс"],
              ["Проверки", stats.data.sample_count],
            ].map(([label, value]) => (
              <article key={label}>
                <span>{label}</span>
                <strong>
                  {stats.data!.sample_count || label === "Проверки"
                    ? value
                    : "—"}
                </strong>
              </article>
            ))}
          </section>
          <p>Доступность — доля успешных проверок за период.</p>
          <section className="chart">
            <h2>Задержка p95 · UTC</h2>
            {stats.data.series.length ? (
              <div className="series">
                {stats.data.series.map((b) => (
                  <div
                    key={b.bucket}
                    title={when(b.bucket) + " · " + b.p95_latency_ms + " мс"}
                  >
                    <span>{Math.round(b.p95_latency_ms)}</span>
                    <i
                      style={{
                        height: Math.max(
                          3,
                          (90 * b.p95_latency_ms) /
                            Math.max(
                              1,
                              ...stats.data!.series.map(
                                (v) => v.p95_latency_ms,
                              ),
                            ),
                        ),
                      }}
                    />
                    <small>{b.bucket.slice(5, 10)}</small>
                  </div>
                ))}
              </div>
            ) : (
              <p>Ожидается первая проверка.</p>
            )}
          </section>
        </>
      )}
      <section className="card">
        <h2>История проверок</h2>
        <div className="uptime-strip">
          {checks.data?.items
            .slice()
            .reverse()
            .map((c) => (
              <i
                key={c.id}
                className={c.success ? "up" : "fail"}
                title={when(c.checked_at) + " · " + (c.error_type ?? "OK")}
              />
            ))}
        </div>
        {checks.data?.items.map((c) => (
          <p key={c.id}>
            {when(c.checked_at)} · {c.success ? "Успех" : c.error_type} · HTTP{" "}
            {c.http_status ?? "—"} · {c.latency_ms} мс
          </p>
        ))}
      </section>
      <section className="card">
        <h2>Инциденты</h2>
        {inc.data?.items.length ? (
          inc.data.items.map((i) => (
            <p key={i.id}>
              {when(i.started_at)} — {i.ended_at ? when(i.ended_at) : "открыт"}{" "}
              · {i.cause}
            </p>
          ))
        ) : (
          <p>За период инцидентов нет.</p>
        )}
      </section>
    </>
  );
}
