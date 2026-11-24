import {useState,useEffect} from "react";
import {Field,Form,Notice} from "./ui";
import {api,type User,type Session} from "./api";
import * as Dialog from "@radix-ui/react-dialog";
export function SettingsPage({
  user,
  refresh,
  end,
}: {
  user: User;
  refresh: () => Promise<void>;
  end: () => void;
}) {
  const [sessions, setSessions] = useState<Session[]>([]),
    [msg, setMsg] = useState(""),
    [chosen, choose] = useState<Session | null>(null);
  async function reload() {
    setSessions((await api<{ items: Session[] }>("/me/sessions")).items);
  }
  useEffect(() => {
    reload().catch((e) => setMsg(e.message));
  }, []);
  return (
    <>
      <header>
        <div>
          <small>SATURNITA / ACCOUNT</small>
          <h1>Настройки</h1>
        </div>
      </header>
      <div className="settings">
        <Form
          title="Профиль"
          label="Сохранить профиль"
          submit={async (d) => {
            await api("/me", "PATCH", Object.fromEntries(d), user.csrf_token);
            await refresh();
          }}
        >
          <p>{user.email}</p>
          <Field
            name="display_name"
            label="Имя"
            initial={user.display_name}
            max={120}
          />
          <Field name="timezone" label="Часовой пояс" initial={user.timezone} />
        </Form>
        <Form
          title="Безопасность"
          label="Сменить пароль"
          submit={async (d) => {
            await api(
              "/me/password",
              "PUT",
              Object.fromEntries(d),
              user.csrf_token,
            );
            await reload();
          }}
        >
          <Field
            name="current_password"
            label="Текущий пароль"
            type="password"
            auto="current-password"
          />
          <Field
            name="new_password"
            label="Новый пароль · минимум 12 символов"
            min={12}
            type="password"
            auto="new-password"
          />
        </Form>
      </div>
      <section className="card">
        <h2>Активные сессии</h2>
        <Notice text={msg} />
        {sessions.map((s) => (
          <div className="session" key={s.id}>
            <div>
              <b>{s.current ? "Текущая сессия" : "Другой вход"}</b>
              <p>{s.user_agent}</p>
            </div>
            <button className="secondary" onClick={() => choose(s)}>
              Завершить
            </button>
          </div>
        ))}
      </section>
      <Dialog.Root
        open={!!chosen}
        onOpenChange={(v) => {
          if (!v) choose(null);
        }}
      >
        <Dialog.Portal>
          <Dialog.Overlay className="overlay" />
          <Dialog.Content className="dialog">
            <Dialog.Title>Завершить сессию?</Dialog.Title>
            <Dialog.Description>
              Для продолжения потребуется войти снова.
            </Dialog.Description>
            <Dialog.Close asChild>
              <button className="secondary">Отмена</button>
            </Dialog.Close>{" "}
            <button
              onClick={async () => {
                if (!chosen) return;
                try {
                  await api(
                    "/me/sessions/" + chosen.id,
                    "DELETE",
                    undefined,
                    user.csrf_token,
                  );
                  if (chosen.current) end();
                  else await reload();
                } catch (e) {
                  setMsg(String(e));
                }
                choose(null);
              }}
            >
              Завершить вход
            </button>
          </Dialog.Content>
        </Dialog.Portal>
      </Dialog.Root>
    </>
  );
}
