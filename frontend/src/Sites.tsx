import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { api, type User } from "./api";
import { Form, Field } from "./ui";
import {
  Action,
  Copy,
  Feedback,
  useData,
  type Site,
  type Page,
} from "./product";
import { ReportView } from "./Reports";
export function SiteReport() {
  const { id } = useParams();
  return (
    <ReportView
      site
      title="Аналитика сайта"
      path={"/sites/" + id + "/analytics"}
    />
  );
}
export function Sites({ user }: { user: User }) {
  const list = useData<Page<Site>>("/sites");
  const [edit, select] = useState<Site | null>(null),
    [nonce, reset] = useState(0);
  return (
    <>
      <header>
        <h1>Сайты</h1>
      </header>
      <Form
        key={edit?.id || nonce}
        title={edit ? "Настройки сайта" : "Подключить сайт"}
        label="Сохранить сайт"
        submit={async (d) => {
          await api(
            edit ? "/sites/" + edit.id : "/sites",
            edit ? "PATCH" : "POST",
            {
              name: d.get("name"),
              hostname: d.get("hostname"),
              timezone: d.get("timezone"),
              origins: String(d.get("origins")).split(/\s+/).filter(Boolean),
              status: d.get("status"),
            },
            user.csrf_token,
          );
          select(null);
          reset((n) => n + 1);
          list.reload();
        }}
      >
        <Field name="name" label="Название" initial={edit?.name} />
        <Field name="hostname" label="Домен сайта" initial={edit?.hostname} />
        <Field
          name="timezone"
          label="Часовой пояс"
          initial={edit?.timezone || "UTC"}
        />
        <Field
          name="origins"
          label="Разрешённые origins через пробел"
          initial={edit?.origins.join(" ")}
        />
        <label>
          Сбор событий
          <select name="status" defaultValue={edit?.status || "active"}>
            <option value="active">Включён</option>
            <option value="disabled">Приостановлен</option>
          </select>
        </label>
      </Form>
      {edit && <button onClick={() => select(null)}>Отмена</button>}
      <Feedback error={list.error} loading={!list.data} />
      {list.data && !list.data.items.length && (
        <p>Подключённых сайтов пока нет.</p>
      )}
      {list.data?.items.map((s) => {
        const snippet =
          '<script defer src="' +
          location.origin +
          '/tracker.js" data-site-id="' +
          s.public_id +
          '"></script>';
        return (
          <section className="card resource" key={s.id}>
            <h2>{s.name}</h2>
            <p>
              {s.hostname} · {s.status}
            </p>
            <code>{snippet}</code>
            <div className="actions">
              <Copy value={snippet} />
              <Link to={"/app/sites/" + s.id}>Отчёт сайта</Link>
              {s.origins.includes(location.origin) && <a href={"/tracker-demo.html?site_id="+s.public_id} target="_blank" rel="noreferrer">Проверить трекер</a>}
              <button onClick={() => select(s)}>Настройки сайта</button>
              <Action
                run={async () => {
                  if (confirm("Удалить сайт и его события навсегда?")) {
                    await api(
                      "/sites/" + s.id,
                      "DELETE",
                      undefined,
                      user.csrf_token,
                    );
                    list.reload();
                  }
                }}
              >
                Удалить сайт
              </Action>
            </div>
          </section>
        );
      })}
    </>
  );
}
