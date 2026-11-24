import { test, expect } from "@playwright/test";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
test("персональный домен, лимит переходов и настоящие события сайта", async ({
  page,
  request,
  baseURL,
}) => {
  const token = crypto.randomUUID().slice(0, 8),
    label = "web-" + token,
    root = baseURL!,
    headers = { Origin: root };
  const register = await page.request.post("/api/v1/auth/register", {
    headers,
    data: {
      email: token + "@product.test",
      display_name: "Аналитик",
      password: "test password for pulse",
    },
  });
  expect(register.status()).toBe(201);
  const me = await (await page.request.get("/api/v1/me")).json();
  const auth = { Origin: root, "X-CSRF-Token": me.csrf_token };
  const create = async (path: string, data: unknown) => {
    const r = await page.request.post("/api/v1/" + path, {
      headers: auth,
      data,
    });
    expect(r.status(), await r.text()).toBe(201);
    return r.json();
  };
  const domain = await create("domains/subdomains", { label });
  const link = await create("links", {
    domain_id: domain.id,
    name: "Кампания",
    slug: "promo",
    target_url: "https://example.org/?ref=a%2Fb&ref=c&utm_source=old#demo",
    utm: { source: "mail" },
    click_limit: 2,
  });
  const redirectURL = new URL(link.short_url);
  const duplicate = await page.request.post("/api/v1/links",{headers:auth,data:{domain_id:domain.id,slug:"promo",name:"Повтор",target_url:"https://example.org/a"}});
  expect(duplicate.status()).toBe(409);
  const internal = await page.request.post("/api/v1/links",{headers:auth,data:{domain_id:domain.id,slug:"chain",name:"Цепочка",target_url:link.short_url}});
  expect(internal.status()).toBe(422);
  const other = await request.post("/api/v1/auth/register",{headers,data:{email:"other-"+token+"@product.test",display_name:"Другой владелец",password:"test password for pulse"}});
  expect(other.status()).toBe(201);
  expect((await request.get("/api/v1/links/"+link.id)).status()).toBe(404);
  expect((await request.get("/api/v1/domains/"+domain.id)).status()).toBe(404);
  expect((await request.head(root+"/promo",{headers:{Host:redirectURL.host},maxRedirects:0})).status()).toBe(302);
  expect((await request.get(root+"/promo",{headers:{Host:"unknown.invalid"},maxRedirects:0})).status()).toBe(404);
  // Loopback transport with the registered authority also works on systems without wildcard DNS.
  const visit = () =>
    request.get(root + redirectURL.pathname + "?extra=ignored", {
      maxRedirects: 0,
      headers: { Host: redirectURL.host },
    });
  const responses = await Promise.all([visit(), visit(), visit()]);
  expect(responses.map((r) => r.status()).sort()).toEqual([302, 302, 410]);
  expect(responses.find((r) => r.status() === 302)?.headers().location).toBe(
    "https://example.org/?ref=a%2Fb&ref=c&utm_source=mail#demo",
  );
  await page.goto("/app/links");
  await expect(page.getByRole("heading", { name: "Кампания" })).toBeVisible();
  await page.getByRole("link", { name: "Статистика" }).click();
  await expect(page.getByText("mail", { exact: true })).toBeVisible();
  const report = await page.request.get(
    "/api/v1/links/" + link.id + "/analytics",
  );
  expect(report.status(), await report.text()).toBe(200);
  expect((await report.json()).total).toBe(2);
  await page.goto("/app/links");
  await page.getByLabel("Название",{exact:true}).fill("Ссылка из формы");
  await page.getByLabel("Целевой URL").fill("https://example.org/new");
  await page.getByRole("button",{name:"Сохранить ссылку"}).click();
  const createdCard=page.locator("article.resource").filter({has:page.getByRole("heading",{name:"Ссылка из формы"})});
  await expect(createdCard).toBeVisible();
  await createdCard.getByRole("button",{name:"Архивировать"}).click();
  page.once("dialog",dialog=>dialog.accept());
  await createdCard.getByRole("button",{name:"Удалить навсегда"}).click();
  await expect(createdCard).toHaveCount(0);
  let snippet = "";
  const server = createServer((_req, res) => {
    res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
    res.end(snippet);
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  try {
    const siteOrigin =
      "http://localhost:" + (server.address() as AddressInfo).port;
    const site = await create("sites", {
      name: "Учебный сайт",
      hostname: "localhost",
      timezone: "UTC",
      origins: [siteOrigin],
    });
    const fixture = await page.context().newPage();
    expect((await request.get("/api/v1/sites/"+site.id+"/analytics")).status()).toBe(404);
    snippet =
      '<script defer src="' +
      root +
      '/tracker.js" data-site-id="' +
      site.public_id +
      '"></script><h1>Демонстрационный сайт</h1>';
    fixture.on("requestfailed", (r) =>
      console.warn(r.url(), r.failure()?.errorText),
    );
    fixture.on("console", (m) => {
      if (m.type() === "error") console.warn(m.text());
    });
    const collected = () =>
      fixture.waitForResponse(
        (r) => r.url().includes("/collect?") && r.request().method() === "POST",
        { timeout: 8000 },
      );
    const first = collected();
    await fixture.goto(siteOrigin + "/welcome?secret=hidden&utm_source=demo");
    expect((await first).status()).toBe(204);
    const second = collected();
    await fixture.evaluate(() =>
      history.pushState({}, "", "/catalog?token=hidden"),
    );
    expect((await second).status()).toBe(204);
    const third = collected();
    await fixture.evaluate(() => {
      (
        window as Window & {
          Saturnita: { track: (n: string, p: object) => void };
        }
      ).Saturnita.track("signup_completed", { plan: "free" });
    });
    expect((await third).status()).toBe(204);
    const web = await page.request.get(
      "/api/v1/sites/" + site.id + "/analytics",
    );
    expect(web.status(), await web.text()).toBe(200);
    const stats = await web.json();
    expect(stats.total).toBe(2);
    expect(stats.daily_unique_visitors_sum).toBe(1);
    expect(stats.events[0].key).toBe("signup_completed");
    expect(JSON.stringify(stats)).not.toContain("hidden");
    const denied = await request.post(
      "/api/v1/collect?site_id=" + site.public_id,
      { headers: { Origin: "http://unregistered.test" }, data: {} },
    );
    expect(denied.status()).toBe(403);
    await page.goto("/app/sites/" + site.id);
    await expect(
      page.getByText("signup_completed", { exact: true }).first(),
    ).toBeVisible();
    await page.request.patch("/api/v1/links/" + link.id, {
      headers: auth,
      data: { status: "archived" },
    });
    expect((await visit()).status()).toBe(410);
    expect(
      (
        await page.request.delete(
          "/api/v1/links/" + link.id + "?confirm=true",
          { headers: auth },
        )
      ).status(),
    ).toBe(204);
    expect(
      (
        await page.request.delete("/api/v1/domains/" + domain.id, {
          headers: auth,
        })
      ).status(),
    ).toBe(204);
    expect(
      (
        await page.request.delete("/api/v1/sites/" + site.id, { headers: auth })
      ).status(),
    ).toBe(204);
  } finally {
    server.closeAllConnections();
    server.close();
  }
});
