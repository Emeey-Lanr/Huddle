import type { APIRoute } from "astro";
import { apiFetch } from "../../lib/api";

// Quantity changes on the bag page. Plain forms, so they work without
// JavaScript and on bad connections.
export const POST: APIRoute = async ({ request, locals, redirect }) => {
  const form = await request.formData().catch(() => new FormData());
  const id = String(form.get("product_id") ?? "");
  const quantity = Number(form.get("quantity"));
  if (!id || !Number.isInteger(quantity) || quantity < 0) return redirect("/bag?error=1", 303);

  const path = `/cart/items/${encodeURIComponent(id)}`;
  const headers = { "Content-Type": "application/json", cookie: request.headers.get("cookie") ?? "" };
  const res =
    quantity === 0
      ? await apiFetch(locals.host, path, { method: "DELETE", headers })
      : await apiFetch(locals.host, path, { method: "PUT", headers, body: JSON.stringify({ quantity }) });

  return redirect(res.ok ? "/bag" : "/bag?error=1", 303);
};
