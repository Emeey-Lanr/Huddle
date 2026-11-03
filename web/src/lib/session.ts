import type { AstroCookies } from "astro";
import { shopOrigin } from "./api";

// A signed-in shopper's API token lives in a cookie only this shop's server
// can read: httpOnly, so page scripts never see it, and set per shop address,
// so one shop's account never reaches another. The API's tokens last an hour.
const name = "customer_token";
const maxAge = 55 * 60;

export const readToken = (cookies: AstroCookies) => cookies.get(name)?.value ?? "";

export function saveToken(cookies: AstroCookies, request: Request, token: string) {
  cookies.set(name, token, {
    path: "/",
    httpOnly: true,
    sameSite: "lax",
    secure: shopOrigin(request).startsWith("https:"),
    maxAge,
  });
}

export const clearToken = (cookies: AstroCookies) => cookies.delete(name, { path: "/" });

// Where to go after signing in: only a path on this shop, never another site.
export function nextPath(value: string | null) {
  return value && value.startsWith("/") && !value.startsWith("//") && !value.startsWith("/\\") ? value : "/account";
}
