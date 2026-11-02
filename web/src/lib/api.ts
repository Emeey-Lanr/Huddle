// Talks to the API from the website's server. Browsers never call the API
// directly: they call /api/* here, so the cart cookie is first-party.

export type Theme = {
  logo_url: string;
  banner_url: string;
  primary_color: string;
  accent_color: string;
  font: "anton" | "bricolage" | "fraunces" | "system" | "inter" | "poppins" | "lora";
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

export type CartItem = {
  product_id: string;
  product_name: string;
  product_price: string;
  currency: string;
  quantity: number;
};

export type Cart = { items: CartItem[] | null };

export type Order = {
  id: string;
  status: "pending" | "paid" | "fulfilled" | "cancelled" | "expired";
  currency: string;
  total: string;
  name: string;
  phone: string;
  email: string;
  delivery_method: "delivery" | "pickup";
  address: string;
  city: string;
  state: string;
  note: string;
  items: { product_id: string; product_name: string; unit_price: string; quantity: number; line_total: string }[];
  expires_at: string;
};

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

export const getCart = (host: string, cookie: string) => apiData<Cart>(host, "/cart", { headers: { cookie } });

export const getOrder = (host: string, id: string) =>
  apiData<Order>(host, `/storefront/orders/${encodeURIComponent(id)}`);

export const placeOrder = (host: string, cookie: string, details: Record<string, string>) =>
  apiData<Order>(host, "/storefront/checkout", {
    method: "POST",
    headers: { "Content-Type": "application/json", cookie },
    body: JSON.stringify(details),
  });

// The shop's own address, as the shopper sees it, for Bachs to send them back to.
export function shopOrigin(request: Request) {
  const proto = request.headers.get("x-forwarded-proto") ?? new URL(request.url).protocol.replace(":", "");
  return `${proto}://${request.headers.get("host")}`;
}

// Opens a Bachs checkout for the order and returns the page to send the shopper to.
export async function startPayment(host: string, orderId: string, request: Request) {
  const receipt = `${shopOrigin(request)}/orders/${encodeURIComponent(orderId)}`;
  const payment = await apiData<{ checkout_url: string }>(host, `/storefront/orders/${encodeURIComponent(orderId)}/pay`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ success_url: `${receipt}?check=1`, cancel_url: receipt }),
  });
  return payment.checkout_url;
}
