import type { APIRoute } from "astro";
import { clearToken } from "../../lib/session";

// The API's tokens can't be revoked; dropping the cookie is what signs out here.
export const POST: APIRoute = ({ cookies, redirect }) => {
  clearToken(cookies);
  return redirect("/", 303);
};
