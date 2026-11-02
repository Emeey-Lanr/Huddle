import { defineMiddleware } from "astro:middleware";
import { ApiError, countItems, getCart, getStore } from "./lib/api";

// The address decides the store. Only bare local or tunnel addresses, which
// can't name one, fall back to DEV_STORE_HOST.
function storeHost(request: Request) {
  const host = request.headers.get("host") ?? "";
  const bare = /^(localhost|127\.0\.0\.1)(:\d+)?$/.test(host) || host.endsWith(".ngrok-free.dev");
  return (bare && process.env.DEV_STORE_HOST) || host;
}

// The cart cookie is first-party here, so another site must not be able to
// submit forms for a shopper. Astro's own check compares full origins, which
// breaks behind a tunnel, so this compares hosts, on every store's address.
function fromOtherSite(request: Request) {
  if (request.method === "GET" || request.method === "HEAD") return false;
  const origin = request.headers.get("origin");
  if (!origin) return true;
  try {
    return new URL(origin).host !== request.headers.get("host");
  } catch {
    return true;
  }
}

export const onRequest = defineMiddleware(async (context, next) => {
  if (fromOtherSite(context.request)) return new Response("Forbidden", { status: 403 });

  const host = storeHost(context.request);
  context.locals.host = host;
  if (context.url.pathname.startsWith("/api/")) return next();

  try {
    context.locals.store = await getStore(host);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      return new Response("This shop doesn't exist.", { status: 404, headers: { "Content-Type": "text/plain" } });
    }
    throw err;
  }

  context.locals.bagCount = 0;
  const cookie = context.request.headers.get("cookie") ?? "";
  if (cookie.includes("cart_session=")) {
    const cart = await getCart(host, cookie).catch(() => null);
    if (cart) context.locals.bagCount = countItems(cart);
  }
  return next();
});
