import {Link,useNavigate} from "react-router-dom";
import {Brand,Form,Field} from "./ui";
import {api} from "./api";
import {demoEnabled} from "./config";
export function Auth({
  register,
  refresh,
}: {
  register: boolean;
  refresh: () => Promise<void>;
}) {
  const nav = useNavigate();
  return (
    <main className="auth">
      <div className="authbox">
        <Brand />
        <Form
          title={register ? "Создать аккаунт" : "Войти в кабинет"}
          label={register ? "Зарегистрироваться" : "Войти"}
          submit={async (data) => {
            await api(
              "/auth/" + (register ? "register" : "login"),
              "POST",
              Object.fromEntries(data),
            );
            await refresh();
            nav("/app", { replace: true });
          }}
        >
          {register && (
            <Field name="display_name" label="Имя" max={120} auto="name" />
          )}
          <Field name="email" label="Email" type="email" auto="email" />
          <Field
            name="password"
            label={register ? "Пароль · минимум 12 символов" : "Пароль"}
            min={register ? 12 : 1}
            type="password"
            auto={register ? "new-password" : "current-password"}
          />
        </Form>
        <Link to={register ? "/login" : "/register"}>
          {register ? "Уже есть аккаунт? Войти" : "Создать аккаунт"}
        </Link>
        {demoEnabled && <Link to="/demo">Демонстрация Overview ↗</Link>}
      </div>
    </main>
  );
}
