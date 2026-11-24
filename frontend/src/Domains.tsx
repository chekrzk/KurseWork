import { Form, Field } from "./ui";
import {
  Action,
  Copy,
  Feedback,
  useData,
  type Domain,
  type Page,
} from "./product";
import { api, type User } from "./api";
export function Domains({ user }: { user: User }) {
  const { data, error, reload } = useData<Page<Domain>>("/domains");
  const post = async (path: string, d?: unknown) => {
    try {
      await api("/domains/" + path, "POST", d, user.csrf_token);
    } finally {
      reload();
    }
  };
  return (
    <>
      <header>
        <h1>Домены</h1>
      </header>
      <div className="settings">
        <Form
          title="Персональный поддомен"
          label="Занять поддомен"
          submit={(d) => post("subdomains", { label: d.get("label") })}
        >
          <Field name="label" label="Имя поддомена" />
          <p>Один поддомен на аккаунт.</p>
        </Form>
        <Form
          title="Внешний домен"
          label="Получить DNS-инструкцию"
          submit={(d) => post("custom", { hostname: d.get("hostname") })}
        >
          <Field name="hostname" label="Домен для ссылок" />
        </Form>
      </div>
      <Feedback error={error} loading={!data} />
      {data?.items.map((d) => (
        <section key={d.id} className="card resource">
          <h2>{d.hostname}</h2>
          <p>
            {d.kind} · {d.status}
          </p>
          {d.kind === "custom" && d.status !== "active" && (
            <>
              <p>TXT: _saturnita.{d.hostname}</p>
              <code>{d.challenge}</code> <Copy value={d.challenge || ""} />
              <p>CNAME: {d.hostname} → go.saturnita.ru</p>
              <div className="actions">
                <Action run={() => post(d.id + "/verify")}>
                  Проверить TXT
                </Action>
                <Action run={() => post(d.id + "/verification/rotate")}>
                  Обновить код
                </Action>
                {d.status === "verified" && (
                  <Action run={() => post(d.id + "/activate")}>
                    Проверить маршрут
                  </Action>
                )}
              </div>
            </>
          )}
          {d.kind !== "system" && (
            <Action
              run={async () => {
                if (confirm("Удалить домен?")) {
                  await api(
                    "/domains/" + d.id,
                    "DELETE",
                    undefined,
                    user.csrf_token,
                  );
                  reload();
                }
              }}
            >
              Удалить домен
            </Action>
          )}
        </section>
      ))}
    </>
  );
}
