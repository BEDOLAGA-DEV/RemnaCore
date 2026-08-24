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
    port: 3000,
    proxy: {
      // Caddy serves the cabinet vhost on :80 and proxies /api and /sub to
      // remnacore:4000 there. Port 8080 is the Remnawave panel, not this API.
      "/api": {
        target: "http://localhost",
        changeOrigin: true,
      },
      "/sub": {
        target: "http://localhost",
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "dist",
    sourcemap: true,
  },
});
