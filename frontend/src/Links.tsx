import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, type User } from "./api";
import {
  Action,
  Feedback,
  Copy,
  useData,
  type Page,
  type Domain,
  type ShortLink,
} from "./product";
import { Form, Field } from "./ui";
import { ReportView } from "./Reports";
export function LinkReport() {
  const { id } = useParams();
  return (
    <ReportView path={"/links/" + id + "/analytics"} title="Аналитика ссылки" />
  );
}
export function Links({ user }: { user: User }) {
  const [sort, setSort] = useState("new");
  const [query, setQuery] = useState(""),
    [status, setStatus] = useState(""),
    [cursor, setCursor] = useState(""),
    [editing, edit] = useState<ShortLink | null>(null),
    [nonce, reset] = useState(0);
  const domains = useData<Page<Domain>>("/domains"),
    list = useData<Page<ShortLink>>(
      "/links?" + new URLSearchParams({ query, status, cursor, sort }),
    );
  function reload() {
    setCursor("");
    list.reload();
  }
  return (
    <>
      <header>
        <h1>Ссылки</h1>
      </header>
      <Form
        key={editing?.id || nonce}
        title={editing ? "Редактирование" : "Новая короткая ссылка"}
        label="Сохранить ссылку"
        disabled={!editing && !domains.data?.items.some((domain) => domain.status === "active")}
        submit={async (d) => {
          const utm = Object.fromEntries(
            ["source", "medium", "campaign", "term", "content"]
              .map((k) => [k, String(d.get(k) || "")])
              .filter(([, v]) => v),
          );
          const data = {
            monitor_target:d.get("monitor_target")==="on",
            name: d.get("name"),
            target_url: d.get("target_url"),
            utm,
            click_limit: d.get("limit") ? Number(d.get("limit")) : null,
            expires_at: d.get("expires")
              ? new Date(String(d.get("expires"))).toISOString()
              : null,
            ...(editing
              ? { status: d.get("status") }
              : { domain_id: d.get("domain"), slug: d.get("slug") }),
          };
          await api(
            editing ? "/links/" + editing.id : "/links",
            editing ? "PATCH" : "POST",
            data,
            user.csrf_token,
          );
          edit(null);
          reset((n) => n + 1);
          reload();
        }}
      >
        <Field label="Название" name="name" initial={editing?.name} />
        <label><input type="checkbox" name="monitor_target" defaultChecked={!!editing?.monitor_id}/>Мониторинг целевого URL</label>
        <Field
          label="Целевой URL"
          name="target_url"
          type="url"
          initial={editing?.target_url}
        />
        {editing ? (
          <label>
            Статус
            <select name="status" defaultValue={editing.status}>
              <option value="active">Активна</option>
              <option value="inactive">Отключена</option>
              <option value="archived">В архиве</option>
            </select>
          </label>
        ) : (
          <div className="filters">
            <label>
              Домен
              <select required name="domain">
                {domains.data?.items
                  .filter((d) => d.status === "active")
                  .map((d) => (
                    <option key={d.id} value={d.id}>
                      {d.hostname}
                    </option>
                  ))}
              </select>
            </label>
            <label>
              Короткий путь
              <input name="slug" placeholder="Автоматически" maxLength={100} />
            </label>
          </div>
        )}
        <details>
          <summary>UTM, срок действия и лимит переходов</summary>
          <div className="filters">
            {["source", "medium", "campaign", "term", "content"].map((k) => (
              <label key={k}>
                utm_{k}
                <input
                  name={k}
                  maxLength={255}
                  defaultValue={editing?.utm[k as keyof ShortLink["utm"]]}
                />
              </label>
            ))}
            <label>
              Лимит
              <input
                name="limit"
                type="number"
                min={1}
                defaultValue={editing?.click_limit ?? ""}
              />
            </label>
            <label>
              Действует до
              <input
                type="datetime-local"
                name="expires"
                defaultValue={
                  editing?.expires_at
                    ? new Date(
                        Date.parse(editing.expires_at) -
                          new Date(editing.expires_at).getTimezoneOffset() *
                            60000,
                      )
                        .toISOString()
                        .slice(0, 16)
                    : ""
                }
              />
            </label>
          </div>
        </details>
      </Form>
      {editing && <button onClick={() => edit(null)}>Отмена</button>}
      <div className="filters">
        <label>
          Сортировка
          <select
            value={sort}
            onChange={(e) => {
              setSort(e.target.value);
              setCursor("");
            }}
          >
            <option value="new">Новые</option>
            <option value="clicks">Больше переходов</option>
            <option value="name">По названию</option>
          </select>
        </label>
        <label>
          Поиск
          <input
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setCursor("");
            }}
          />
        </label>
        <label>
          Статус
          <select
            value={status}
            onChange={(e) => {
              setStatus(e.target.value);
              setCursor("");
            }}
          >
            <option value="">Все</option>
            {["active", "inactive", "archived"].map((s) => (
              <option key={s}>{s}</option>
            ))}
          </select>
        </label>
      </div>
      <Feedback error={domains.error || list.error} loading={!list.data} />
      {list.data?.items.map((l) => (
        <article className="card resource" key={l.id}>
          <h2>{l.name}</h2>
          <a href={l.short_url} target="_blank" rel="noreferrer">
            {l.short_url}
          </a>
          <p>{l.target_url}</p>
          <p>
            {l.status} · Переходы: {l.click_count}
          </p>
          <div className="actions">
            <Copy value={l.short_url} />
            <Link to={"/app/links/" + l.id}>Статистика</Link>
            <button
              onClick={() => {
                edit(l);
                window.scrollTo(0, 0);
              }}
            >
              Изменить
            </button>
            {l.status !== "archived" ? (
              <Action
                run={async () => {
                  await api(
                    "/links/" + l.id,
                    "PATCH",
                    { status: "archived" },
                    user.csrf_token,
                  );
                  reload();
                }}
              >
                Архивировать
              </Action>
            ) : (
              <Action
                run={async () => {
                  if (confirm("Удалить ссылку и историю переходов навсегда?")) {
                    await api(
                      "/links/" + l.id + "?confirm=true",
                      "DELETE",
                      undefined,
                      user.csrf_token,
                    );
                    reload();
                  }
                }}
              >
                Удалить навсегда
              </Action>
            )}
          </div>
        </article>
      ))}
      {list.data && !list.data.items.length && (
        <p>Ссылок по этому запросу нет.</p>
      )}
      {list.data?.next_cursor && (
        <button onClick={() => setCursor(list.data!.next_cursor!)}>
          Дальше
        </button>
      )}
      {cursor && <button onClick={() => setCursor("")}>К началу</button>}
    </>
  );
}
