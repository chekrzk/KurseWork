import { useState, useEffect } from "react";
import { Activity, Link2, Globe, Radio, MousePointer2 } from "lucide-react";

import { demoEnabled } from "./config";
import { useData, Feedback, type Report } from "./product";
export function Overview({ demo = false }: { demo?: boolean }) {
  const [sample, setSample] = useState<typeof import("./demo").sample>();
  const live = useData<Report>(demo ? "" : "/dashboard/overview");
  const values = sample?.values ?? live.data?.series.map((p) => p.value) ?? [];
  const top =
    sample?.top ?? live.data?.pages.map((p) => [p.key, p.value] as const) ?? [];
  const events =
    sample?.events ??
    live.data?.recent.map(
      (r) => r.path + " · " + new Date(r.t).toLocaleString(),
    ) ??
    [];
  useEffect(() => {
    if (demoEnabled && demo) import("./demo").then((m) => setSample(m.sample));
  }, [demo]);
  const cards = [
    {
      name: "Переходы · 7 дней",
      icon: MousePointer2,
      value: sample?.clicks ?? live.data?.total,
    },
    {
      name: "Активные ссылки",
      icon: Link2,
      value: sample?.links ?? live.data?.active_links,
    },
    { name: "Сайты", icon: Globe, value: sample?.sites ?? live.data?.sites },
    { name: "Мониторы", icon: Radio, value: sample?.monitors??live.data?.active_monitors },
  ];
  return (
    <>
      <header>
        <div>
          <small>SATURNITA / OVERVIEW</small>
          <h1>Обзор</h1>
        </div>
        {demo && <span className="pill">Демонстрация</span>}
      </header>
      <section className="metrics">
        {!demo && <Feedback error={live.error} loading={!live.data} />}
        {cards.map(({ name, icon: Icon, value }) => (
          <article key={name}>
            <Icon size={18} />
            <span>{name}</span>
            <strong>{value ?? "—"}</strong>
          </article>
        ))}
      </section>
      <section className="chart">
        <div>
          <small>LINK ANALYTICS</small>
          <h2>Динамика переходов</h2>
        </div>
        {values.length ? (
          <>
            <div className="bars" aria-label="Переходы по дням">
              {values.map((n, i) => (
                <div key={i}>
                  <strong>{n}</strong>
                  <i style={{ height: (104 * n) / Math.max(...values) }} />
                  <span>
                    {sample
                      ? ["Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"][i]
                      : live.data?.series[i].bucket.slice(5, 10)}
                  </span>
                </div>
              ))}
            </div>
          </>
        ) : (
          <div className="empty">
            <Activity size={38} />
            <h3>Переходов пока нет</h3>
            <p>Здесь появится график активности ваших ссылок.</p>
          </div>
        )}
      </section>
      <section className="panel">
        <Radio />
        <div>
          <h2>Мониторинг</h2>
          <p>
            {sample ? "Демонстрационные мониторы активны" : live.data?.monitor_alerts.length?"Требуют внимания":live.data?.active_monitors?"Активных предупреждений нет":"Проверок пока нет"}
          </p>
          {live.data?.monitor_alerts.map(m=><p key={m.id}><a href={"/app/monitors/"+m.id}>{m.name} · {m.status}</a></p>)}
        </div>
      </section>
      <div className="overview-detail">
        <section className="card">
          <h2>Популярные ссылки</h2>
          {top.length ? (
            <table>
              <thead>
                <tr>
                  <th>Ссылка</th>
                  <th>Переходы</th>
                </tr>
              </thead>
              <tbody>
                {top.map(([slug, n]) => (
                  <tr key={slug}>
                    <td>{slug}</td>
                    <td>{n}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <p>Ссылок с переходами пока нет.</p>
          )}
        </section>
        <section className="card">
          <h2>Последние события</h2>
          {events.length ? (
            <ul>
              {events.map((event, i) => (
                <li key={i}>{event}</li>
              ))}
            </ul>
          ) : (
            <p>История событий пуста.</p>
          )}
        </section>
      </div>
    </>
  );
}
