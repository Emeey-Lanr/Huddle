import type { APIRoute } from "astro";
import { startPayment } from "../../lib/api";

// "Pay now" on an order that hasn't been paid yet.
export const POST: APIRoute = async ({ request, locals, redirect }) => {
  const form = await request.formData().catch(() => new FormData());
  const id = String(form.get("order_id") ?? "");
  if (!id) return redirect("/", 303);
  try {
    return redirect(await startPayment(locals.host, id, request), 303);
  } catch {
    return redirect(`/orders/${encodeURIComponent(id)}?error=1`, 303);
  }
};
