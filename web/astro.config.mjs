import node from "@astrojs/node";
import { defineConfig } from "astro/config";

// Every page is rendered per request, because the store comes from the address.
export default defineConfig({
  output: "server",
  adapter: node({ mode: "standalone" }),
  devToolbar: { enabled: false },
  vite: { server: { allowedHosts: [".localhost", ".ngrok-free.dev"] } },
});
