import type { APIRoute } from "astro";
import { apiFetch, countItems } from "../../lib/api";

// Adds to the bag through the API and passes the cart cookie back to the browser.
export const POST: APIRoute = async ({ request, locals }) => {
  const input = await request.json().catch(() => null);
  if (typeof input?.product_id !== "string") {
    return Response.json({ message: "Pick a product first" }, { status: 400 });
  }

  const res = await apiFetch(locals.host, "/cart", {
    method: "POST",
    headers: { "Content-Type": "application/json", cookie: request.headers.get("cookie") ?? "" },
    body: JSON.stringify({ product_id: input.product_id, quantity: 1 }),
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    return Response.json({ message: body?.message ?? "Couldn't add that to your bag" }, { status: res.status });
  }

  const headers = new Headers({ "Content-Type": "application/json" });
  for (const cookie of res.headers.getSetCookie()) headers.append("Set-Cookie", cookie);
  return new Response(JSON.stringify({ count: countItems(body.data) }), { headers });
};
