import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import tailwind from "@tailwindcss/vite";
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const proxy = Object.fromEntries(
    ["/api", "/healthz", "/readyz"].map((p) => [
      p,
      { target: env.API_TARGET || "http://127.0.0.1:8080" },
    ]),
  );
  return {
    plugins: [react(), tailwind()],
    server: { host: "0.0.0.0", port: 5173, strictPort: true, proxy },
    preview: { host: "0.0.0.0", port: 5173, strictPort: true, proxy },
  };
});
