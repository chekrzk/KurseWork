import type { components } from "./generated";
export type User = components["schemas"]["Me"];
export type Session = components["schemas"]["Session"];
export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}
export async function api<T>(
  path: string,
  method = "GET",
  data?: unknown,
  csrf?: string,
): Promise<T> {
  const res = await fetch("/api/v1" + path, {
    method,
    credentials: "same-origin",
    headers: {
      ...(data ? { "Content-Type": "application/json" } : {}),
      ...(csrf ? { "X-CSRF-Token": csrf } : {}),
    },
    body: data ? JSON.stringify(data) : undefined,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    if (res.status === 401 && path != "/me" && !path.startsWith("/auth/"))
      window.dispatchEvent(new Event("session-ended"));
    throw new ApiError(
      res.status,
      body?.error?.message || "Не удалось выполнить запрос",
    );
  }
  return res.status === 204 ? (undefined as T) : res.json();
}
