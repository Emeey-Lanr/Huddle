import node from "@astrojs/node";
import { defineConfig } from "astro/config";

// Every page is rendered per request, because the store comes from the address.
export default defineConfig({
  output: "server",
  adapter: node({ mode: "standalone" }),
  devToolbar: { enabled: false },
  security: { checkOrigin: false }, // replaced by fromOtherSite in src/middleware.ts
  vite: {
    server: {
      allowedHosts: [".localhost", ".ngrok-free.dev"],
      // Development only: one ngrok address serves the website and Bachs webhooks.
      proxy: { "/webhooks": "http://localhost:8080" },
    },
  },
});
