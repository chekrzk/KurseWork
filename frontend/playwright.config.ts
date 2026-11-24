import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "tests",
  use: {
    baseURL: process.env.APP_ORIGIN || "http://localhost:5173",
    channel: process.env.BROWSER_CHANNEL || undefined,
  },
  projects: [
    { name: "desktop", use: { viewport: { width: 1366, height: 850 } } },
    { name: "mobile", use: { viewport: { width: 390, height: 844 } } },
  ],
});
