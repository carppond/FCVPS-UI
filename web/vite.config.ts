import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import tailwindcss from "@tailwindcss/vite";
import { TanStackRouterVite } from "@tanstack/router-plugin/vite";
import path from "path";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    TanStackRouterVite({ routesDirectory: "./src/routes" }),
    react(),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    // Bind to all interfaces so a phone on the same LAN can scan the QR code
    // and load the share URL (which encodes window.location.host).
    host: true,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
        // SSH terminal / agent WS ride the same /api prefix.
        ws: true,
      },
      // sub-store compatible download path — clients (mihomo / sing-box / etc.)
      // hit this when consuming the share URL displayed in the UI.
      "/download": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      // Short link redirect: /s/<code> → hub redirects to the real subscription.
      // MUST use regex (not "/s" prefix) — a plain "/s" key would also match
      // "/src/main.tsx" and break the vite SPA module loader.
      "^/s/[A-Za-z0-9_-]+$": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      // Embedded install-agent.sh + agent binary download paths.
      "/_app": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/dl": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      // Nezha agent v2 compat endpoint.
      "/api/v1/nezha": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
