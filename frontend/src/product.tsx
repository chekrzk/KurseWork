import { useState, useEffect, type ReactNode } from "react";
import { api } from "./api";
import type { components } from "./product-generated";
export type Domain = components["schemas"]["Domain"];
export type ShortLink = components["schemas"]["Link"];
export type Site = components["schemas"]["Website"];
export interface Page<T> {
  items: T[];
  next_cursor?: string | null;
}
export type Group = { key: string; value: number };
export type Report = components["schemas"]["Report"];
export function useData<T>(path: string) {
  const [state, set] = useState<{ path: string; data?: T; error?: string }>({
      path: "",
    }),
    [version, refresh] = useState(0);
  useEffect(() => {
    let live = true;
    if (path)
      api<T>(path)
        .then((data) => {
          if (live) set({ path, data });
        })
        .catch((e) => {
          if (live) set({ path, error: String(e.message) });
        });
    return () => {
      live = false;
    };
  }, [path, version]);
  return {
    ...(state.path === path ? state : {}),
    reload: () => refresh((n) => n + 1),
  };
}
export function Feedback({
  error,
  loading,
}: {
  error?: string;
  loading: boolean;
}) {
  return error ? (
    <p role="alert">{error}</p>
  ) : loading ? (
    <p role="status">Загрузка…</p>
  ) : null;
}
export function Action({
  children,
  run,
}: {
  children: ReactNode;
  run: () => Promise<unknown>;
}) {
  const [busy, set] = useState(false),
    [msg, notice] = useState("");
  return (
    <span>
      <button
        className="secondary"
        disabled={busy}
        onClick={async () => {
          set(true);
          notice("");
          try {
            await run();
          } catch (e) {
            notice(e instanceof Error ? e.message : String(e));
          } finally {
            set(false);
          }
        }}
      >
        {children}
      </button>
      {msg && <span role="alert">{msg}</span>}
    </span>
  );
}
export function Copy({ value }: { value: string }) {
  const [copied, set] = useState(false);
  return (
    <Action
      run={async () => {
        await navigator.clipboard.writeText(value);
        set(true);
      }}
    >
      {copied ? "Скопировано" : "Копировать"}
    </Action>
  );
}
