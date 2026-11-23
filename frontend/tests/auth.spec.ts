import { test, expect } from "@playwright/test";
test("вход, профиль и завершение сессии", async ({ page }, info) => {
  const email = "browser-" + crypto.randomUUID() + "@example.test";
  await page.goto("/app/settings");
  await expect(page).toHaveURL(/login$/);
  await page.getByRole("link", { name: "Создать аккаунт" }).click();
  await page.getByLabel("Имя", { exact: true }).fill("Тест");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel(/Пароль/).fill("browser test password");
  await page.getByRole("button", { name: "Зарегистрироваться" }).click();
  await expect(page.getByRole("heading", { name: "Обзор" })).toBeVisible();
  await expect(page.getByText("Переходов пока нет")).toBeVisible();
  if (info.project.name === "mobile")
    await page.getByRole("button", { name: "Навигация" }).click();
  await page.getByRole("link", { name: "Настройки" }).click();
  await page.getByLabel("Имя", { exact: true }).fill("Новое имя");
  await page.getByRole("button", { name: "Сохранить профиль" }).click();
  await expect(page.getByText("Сохранено")).toBeVisible();
  await page.reload();
  await expect(page.getByLabel("Имя", { exact: true })).toHaveValue(
    "Новое имя",
  );
  await page.getByRole("button", { name: "Завершить", exact: true }).click();
  await page.getByRole("button", { name: "Завершить вход" }).click();
  await expect(page).toHaveURL(/login$/);
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Пароль").fill("browser test password");
  await page.getByRole("button", { name: "Войти" }).click();
  await expect(page).toHaveURL(/app$/);
});
