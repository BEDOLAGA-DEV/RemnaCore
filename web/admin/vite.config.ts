import path from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
      "@shared": path.resolve(import.meta.dirname, "../shared"),
    },
  },
  server: {
    port: 3001,
    proxy: {
      // Caddy serves the admin vhost on :8081 and proxies /api and /readyz to
      // remnacore:4000 there. Port 8080 is the Remnawave panel, and :4000 is
      // not published to the host at all.
      "/api": {
        target: "http://localhost:8081",
        changeOrigin: true,
      },
      "/readyz": {
        target: "http://localhost:8081",
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "dist",
    sourcemap: true,
  },
});
