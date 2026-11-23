import {useState,type FormEvent,type ReactNode} from "react";
import {Orbit} from "lucide-react";

export function Brand() {
  return (
    <span className="brand">
      <Orbit aria-hidden="true" /> Saturnita <b>Pulse</b>
    </span>
  );
}
export function Notice({ text }: { text: string }) {
  return text ? (
    <p role="status" className="notice">
      {text}
    </p>
  ) : null;
}
export function Form({
  title,
  children,
  submit,
  label,
  disabled = false,
}: {
  title: string;
  children: ReactNode;
  submit: (d: FormData) => Promise<void>;
  label: string;
  disabled?: boolean;
}) {
  const [busy, setBusy] = useState(false),
    [msg, setMsg] = useState("");
  async function send(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const d = new FormData(e.currentTarget);
    setBusy(true);
    setMsg("");
    try {
      await submit(d);
      setMsg("Сохранено");
    } catch (e) {
      setMsg(e instanceof Error ? e.message : "Ошибка соединения");
    } finally {
      setBusy(false);
    }
  }
  return (
    <form onSubmit={send}>
      <h2>{title}</h2>
      {children}
      <button disabled={busy || disabled}>{busy ? "Подождите…" : label}</button>
      <Notice text={msg} />
    </form>
  );
}
export function Field({
  name,
  label,
  type = "text",
  initial,
  min,
  auto,
  max,
}: {
  name: string;
  label: string;
  type?: string;
  initial?: string;
  min?: number;
  auto?: string;
  max?: number;
}) {
  return (
    <label>
      {label}
      <input
        required
        name={name}
        type={type}
        defaultValue={initial}
        minLength={min}
        maxLength={max}
        autoComplete={auto}
      />
    </label>
  );
}
