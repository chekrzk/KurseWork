import { test, expect } from "@playwright/test";
test("монитор ссылки, история DNS-проверки и архивация", async ({
  page,
  request,
  baseURL,
}) => {
  const email = crypto.randomUUID() + "@monitor.test";
  const origin = baseURL!;
  expect(
    (
      await page.request.post("/api/v1/auth/register", {
        headers: { Origin: origin },
        data: {
          email,
          password: "test password for pulse",
          display_name: "Мониторинг",
        },
      })
    ).status(),
  ).toBe(201);
  const me = await (await page.request.get("/api/v1/me")).json();
  const headers = { Origin: origin, "X-CSRF-Token": me.csrf_token };
  const domain = (
    await (await page.request.get("/api/v1/domains")).json()
  ).items.find((d: { kind: string }) => d.kind === "system");
  const create = await page.request.post("/api/v1/links", {
    headers,
    data: {
      name: "Проверка цели",
      target_url: "https://absent.invalid/health",
      domain_id: domain.id,
      monitor_target: true,
    },
  });
  expect(create.status(), await create.text()).toBe(201);
  const link = await create.json();
  const path = "/api/v1/monitors/" + link.monitor_id;
  await expect
    .poll(async () => (await (await page.request.get(path)).json()).status, {
      timeout: 30000,
    })
    .toBe("degraded");
  await page.goto("/app/monitors");
  await page.getByRole("link", { name: "Проверка цели" }).click();
  await expect(
    page.getByRole("heading", { name: "История проверок" }),
  ).toBeVisible();
  const stats = await (await page.request.get(path + "/analytics")).json();
  expect(stats.sample_count).toBe(1);
  expect(stats.uptime_percent).toBe(0);
  expect((await request.get(path)).status()).toBe(401);
  expect(
    (
      await page.request.patch("/api/v1/links/" + link.id, {
        headers,
        data: { status: "archived" },
      })
    ).status(),
  ).toBe(200);
  expect((await (await page.request.get(path)).json()).status).toBe("paused");
  expect(
    (await page.request.post(path + "/resume", { headers })).status(),
  ).toBe(409);
  expect(
    (
      await page.request.delete("/api/v1/links/" + link.id + "?confirm=true", {
        headers,
      })
    ).status(),
  ).toBe(204);
  expect((await page.request.get(path)).status()).toBe(404);
});
