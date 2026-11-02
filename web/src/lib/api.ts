// Talks to the API from the website's server. Browsers never call the API
// directly: they call /api/* here, so the cart cookie is first-party.

export type Theme = {
  logo_url: string;
  banner_url: string;
  primary_color: string;
  accent_color: string;
  font: "system" | "inter" | "poppins" | "lora";
  sections: { banner: boolean; about: boolean };
};

export type Store = {
  store_id: string;
  store_name: string;
  store_description: string;
  theme: Theme;
};

export type Product = {
  id: string;
  name: string;
  description: string;
  price: string;
  currency: string;
  stock_quantity: number;
  image_url?: string;
};

export type Cart = { items: { quantity: number }[] | null };

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

const apiUrl = () => process.env.API_URL ?? "http://localhost:8080";

export async function apiFetch(host: string, path: string, init: RequestInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("X-Store-Host", host);
  return fetch(apiUrl() + path, { ...init, headers, signal: AbortSignal.timeout(10_000) });
}

export async function apiData<T>(host: string, path: string, init?: RequestInit): Promise<T> {
  const res = await apiFetch(host, path, init);
  const body = await res.json().catch(() => null);
  if (!res.ok) throw new ApiError(res.status, body?.message ?? "Something went wrong");
  return body.data as T;
}

export const getStore = (host: string) => apiData<Store>(host, "/storefront/store");

export const getProducts = (host: string) =>
  apiData<{ items: Product[] }>(host, "/storefront/products?limit=24&sort=created_at&order=desc");

export const getProduct = (host: string, id: string) =>
  apiData<{ product: Product; images: { img_url: string }[] }>(
    host,
    `/storefront/products/${encodeURIComponent(id)}`,
  );

export const countItems = (cart: Cart) =>
  (cart.items ?? []).reduce((total, item) => total + item.quantity, 0);
