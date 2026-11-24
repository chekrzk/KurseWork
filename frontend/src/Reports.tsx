import { lazy, Suspense, useState } from "react";
const Trend=lazy(()=>import("./ReportCharts").then(m=>({default:m.Trend})));
const Distribution=lazy(()=>import("./ReportCharts").then(m=>({default:m.Distribution})));
import { useData, Feedback, type Group, type Report } from "./product";
function Breakdown({ title, rows }: { title: string; rows: Group[] }) {
  return (
    <section className="card">
      <h2>{title}</h2>
      {(title === "Устройства" || title === "Источники переходов") && (
        <Suspense fallback={<p>Загрузка графика…</p>}><Distribution rows={rows} donut={title === "Устройства"} /></Suspense>
      )}{" "}
      {rows.length ? (
        <ol className="rank">
          {rows.map((r) => (
            <li key={r.key}>
              <span>{r.key}</span>
              <strong>{r.value}</strong>
            </li>
          ))}
        </ol>
      ) : (
        <p>Нет данных за период</p>
      )}
    </section>
  );
}
export function ReportView({
  path,
  title,
  site = false,
}: {
  path: string;
  title: string;
  site?: boolean;
}) {
  const [days, setDays] = useState("7"),
    [start, setStart] = useState(""),
    [end, setEnd] = useState("");
  const [now] = useState(() => new Date());
  const dates =
    days === "custom"
      ? start && end && start <= end
        ? [new Date(start), new Date(Date.parse(end) + 86400000)]
        : null
      : [new Date(+now - Number(days) * 86400000), now];
  const { data, error } = useData<Report>(
    dates
      ? path +
          "?" +
          new URLSearchParams({
            from: dates[0].toISOString(),
            to: dates[1].toISOString(),
          })
      : "",
  );
  return (
    <>
      <header>
        <h1>{title}</h1>
      </header>
      <div className="filters">
        <label>
          Период
          <select value={days} onChange={(e) => setDays(e.target.value)}>
            {[1, 7, 30].map((n) => (
              <option key={n} value={n}>
                {n === 1 ? "24 часа" : n + " дней"}
              </option>
            ))}
            <option value="custom">Выбрать даты</option>
          </select>
        </label>
        {days === "custom" && (
          <>
            <label>
              Начало (UTC)
              <input
                type="date"
                value={start}
                onChange={(e) => setStart(e.target.value)}
              />
            </label>
            <label>
              Конец (UTC)
              <input
                type="date"
                value={end}
                onChange={(e) => setEnd(e.target.value)}
              />
            </label>
          </>
        )}
      </div>
      <Feedback error={error} loading={!!dates && !data} />
      {!dates && <p>Выберите начальную и конечную дату.</p>}
      {data && (
        <>
          <section className="metrics">
            <article>
              {site ? "Просмотры страниц" : "Переходы"}
              <strong>{data.total}</strong>
            </article>
            <article>
              Приблизительные уникальные посетители
              <strong>{data.daily_unique_visitors_sum}</strong>
              <small>
                Сумма дневных значений UTC. Один посетитель может учитываться в
                нескольких днях.
              </small>
            </article>
            <article>
              Предыдущий период<strong>{data.previous}</strong>
              <span>
                {data.change_percent === null
                  ? "Нет базы сравнения"
                  : data.change_percent + "%"}
              </span>
            </article>
          </section>
          <section className="chart">
            <h2>Динамика · {data.timezone}</h2>
            {data.series.length ? (
              <Suspense fallback={<p>Загрузка графика…</p>}><Trend report={data} /></Suspense>
            ) : (
              <p>За выбранный период событий нет.</p>
            )}
          </section>
          <div className="overview-detail">
            <Breakdown
              title={site ? "Популярные страницы" : "Популярные ссылки"}
              rows={data.pages}
            />
            <Breakdown title="Источники переходов" rows={data.sources} />
            <Breakdown title="Устройства" rows={data.devices} />
            <Breakdown title="UTM-кампании" rows={data.campaigns} />
            <Breakdown title="UTM-источники" rows={data.utm_sources} />
            {site && (
              <Breakdown title="Пользовательские события" rows={data.events} />
            )}
          </div>
          <section className="card">
            <h2>Последние события</h2>
            {data.recent.length ? (
              data.recent.map((r, i) => (
                <p key={i}>
                  {new Date(r.t).toLocaleString()} · {r.name} · {r.path} ·{" "}
                  {r.device}
                </p>
              ))
            ) : (
              <p>История пуста.</p>
            )}
          </section>
        </>
      )}
    </>
  );
}
