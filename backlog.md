# Huddle Backlog

Goal: a secure, reliable store platform that at least 200 Nigerian SMEs can use, simple enough that a market woman can run her shop from WhatsApp.

Pick items top to bottom. Each item is done when its "Done when" line is true and `go test ./...` passes.

Database tests use your local Postgres: a separate `huddle_test` database (never the dev `huddle` one), one throwaway schema per test, built from `db/migrations`. Set `TEST_DATABASE_URL` to point elsewhere. If Postgres is unreachable, those tests skip.

## Where we are (updated 23 Oct 2026)

- **Branches:** local `main` has everything through 4.7, 4.4b, 5.1, 5.2 and 5.3: `feat/order-notifications`, `feat/store-domains`, `feat/health-check`, `feat/graceful-shutdown` and `feat/config` (5.3) were merged locally on 23 Oct (`go test ./...` passes), but **`main` is not pushed yet** (`origin/main` is still `42b034e`); pushing it marks the 4.7 PR merged. Next:
  1. Push `main` (`git push origin main`).
  2. Then follow this order (decided 23 Oct; it overrides "top to bottom" for sections 5 and 6), so each step gives something usable:
     1. **Section 5 essentials:** ~~5.2 shutdown + timeouts~~, ~~5.3 config~~ (done), 5.6 Docker, 5.8 deploy + backups (enough to run the API for real).
     2. **6.2 Customer website (storefront template) + theme settings:** sellers can take real orders; the team sets each store's theme by hand at first. Onboard the first real sellers here and learn what they need.
     3. **6.1 phone login + 6.5 WhatsApp bot:** sellers run their shop from WhatsApp.
     4. **AI (6.3, 6.4, 6.6) + 6.7 preview:** the AI takes over the theming and content the team did by hand.
     - 5.4, 5.5, 5.7, 5.9 fit in whenever convenient; 6.9 only once a developer asks.
  3. Look into **2.8** (checkout lock 409 on a new cart) when convenient.
  - Dropped: resetting the Mailtrap password (decided 23 Oct: the Mailtrap sandbox won't be used in production).
- **Sandbox tests passed (23 Oct 2026).** Test store `str-e5564c2ac9824cbdaeb4f0de579d0357` (seller `sandbox1792093898`, password `secret123`, Bachs account `acct_EPpm2BjrPWW263rN`; to get a store key for a new order, call `POST /store/{id}/key` as the seller). Order `ord_e9ab0c2a54704470945fefe7a54d2b0c` (₦15,000, Ankara bag ×1, pickup) was paid on the hosted checkout (test card `4242 4242 4242 4242`); its notification was `sent` on the first attempt 54 s later and the "New paid order" email in Mailtrap showed every detail correctly ✔. That brought the balance up, and a full refund of `ord_fe532f63baea4c23a4da1cf447c25dde` (`rfd_05e84867017e4f3a90ce78f02b126f5d`) went through: `refund.paid` arrived, the payment turned `refunded`, the order `cancelled` and the stock came back ✔. Earlier (15 Oct) the same refund with only ₦14,700 in the balance returned 409 with that balance ✔.
- **Running locally:** start the API (`go run .`, port 8080) and `ngrok http 8080` in **your own terminals**. Claude Code stops background processes when the machine is low on memory (it happened 3 times). The ngrok address has stayed `https://outward-finally-elite.ngrok-free.dev`; if it changes, update both Bachs webhook endpoints. Shoppers pay on the Bachs checkout page by hand, because the Chrome extension has no access to `sandbox-checkout.bachs.io`. Sandbox card: any number except Bachs' decline cards (`4000 0000 0000 0002/9995/0069/0119`), or bank transfer with the page's test controls.
- **Local dev DB** (`huddle`) is at migration **13** (`000013_store_domains`, applied 23 Oct). Apply new migrations with `~/go/bin/migrate -path db/migrations -database "postgres://USER:PASS@localhost:5432/huddle?sslmode=disable" up` (values from `.env`; `.env` uses `KEY = value` with spaces). **Never roll back and re-apply a migration on the dev DB to change it once it holds data**: that wiped saved charge ids on 15 Oct (recovered from ngrok's request log). Add a new migration instead, unless the migration is uncommitted and the data doesn't matter.
- **Bachs:** sandbox keys are in `.env`: `BACHS_API_KEY` (Full Access sandbox key, named `huddle-sandbox`; for live use only Connected Accounts, Payments and Refunds: Write), `BACHS_WEBHOOK_SECRET` (optional `BACHS_BASE_URL`, `BACHS_PLATFORM_FEE_PERCENT`). Bachs webhook endpoints listen to one source each, so the same URL is registered twice (**Connected account events** = `huddle-sellers`, **Account events** = `huddle-platform`) and `BACHS_WEBHOOK_SECRET = secret1,secret2`. Events selected: Account Updated, Capability Updated, Checkout Completed, Checkout Expired, Collection Succeeded, Refund Created/Failed/Paid. Tested in the sandbox on 15 Oct 2026: onboarding created a real sub-account (all capabilities active at once in sandbox, details still "outstanding"), the store showed ready, a ₦15,000 order was paid on the hosted checkout, and `checkout.completed` + `collection.succeeded` arrived through ngrok and marked it paid. `capability.updated` has not been seen yet (sandbox activates capabilities at creation).
- **Email (SMTP):** `.env` points at Mailtrap's sandbox (`SMTP_HOST = sandbox.smtp.mailtrap.io`, `SMTP_PORT = 587`, plus `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM = Huddle <orders@huddle.ng>`). The sandbox delivers nothing to real inboxes: every email, whatever its recipient, lands in the Mailtrap inbox. A test email went through on 21 Oct ✔. Keep each `.env` line as `KEY = value` with no indentation: a bare `KEY` line makes godotenv reject the whole file (the API then exits for lack of `SECRET_KEY`). Never print godotenv's load error in checks: it quotes the file, secrets included.
- **Bachs API notes:** calling it from Python needs a `User-Agent` header (Cloudflare blocks urllib's default with `error code: 1010`). An account's balance is `GET /v1/balances` with `X-Account-Id`. The charge id is `data.charge.id` on `checkout.completed` and `data.charge_id` on `collection.succeeded`. Refunds need a balance that covers them; Bachs' fee is not returned.
- **Decisions made:** guest checkout + optional customer account; delivery details = name, phone, pickup/delivery (+ address/city/state for delivery), optional email and note; stock hold 30 min; sellers can mark orders paid themselves (transfer/cash); **sellers collect** payments in their own Bachs sub-account (direct charges); platform fee 0 for now.

## How we work

- One feature branch per item, merged into `main` when asked (locally or through a GitHub PR). Push to `origin` (GitHub) only when asked.
- **Never add Claude attribution** (no `Co-Authored-By`, no "Generated with") to commits or PRs.
- Every API change updates `doc/swagger.json` (`swagger_test.go` enforces it) and is validated with `python -m openapi_spec_validator doc/swagger.json`.
- New migrations: bump the table count / index list in `pkg/testutil/migrations_test.go`.
- Prove important tests fail without the fix (back up the file to a scratch dir, break it, run, restore — don't use `git checkout` on uncommitted work).
- Never print `.env` secrets. Bachs keys go in `.env`, never in chat.
- `go vet ./...` can be slow on this machine; run it in the background.

---

## 1. Security fixes

- [x] **1.1 JWT secret is empty unless set in the OS environment**
  `pkg/utility/jwt.go:15` reads `SECRET_KEY` before `godotenv.Load()` runs, so tokens are signed with an empty key and anyone can forge one.
  Done when: secret is read after env loads, the app refuses to start without one, and `TestAuthMiddleware_ForgedTokenWithEmptyKeyRejected` passes.

- [x] **1.2 Separate admin and customer tokens**
  Customers get the same token type as admins (`admin_id` claim), so a customer token works on `/product` and `/store` routes.
  Done when: tokens carry a `role` claim, admin routes reject customer tokens, `/customer/profile` rejects admin tokens.

- [x] **1.3 Ownership checks on every product route**
  Only `POST /product` calls `VerifyOwnerShip`. GET, PUT, DELETE and image upload on `/product/:id` let any admin touch any store's products.
  Done when: every admin product route checks the product's store belongs to the caller (403 otherwise).

- [x] **1.4 Cart takes the price from the database, not the client**
  `model.CartItem` accepts `product_price` and `product_name` from the request.
  Done when: cart takes only `product_id` + `quantity`, looks up price/name/stock/store from the DB, and `TestAddToCart_PriceComesFromDatabase` passes.

- [x] **1.5 Turn on the rate limiter**
  `middleware.RateLimiter` exists but is not registered. Apply it at least to signup/signin routes.

---

## 2. Correctness fixes

- [x] **2.1 Product update resets stock to 0**
  `pkg/services/product.go:221` — `StockQuantity >= 0` is always true when the field is omitted.
  Done when: fields left out of the request are not changed (use pointer fields like `Active`).

- [x] **2.2 Failed image save reports success**
  `pkg/helper/product.go:217` — if the DB save fails but Cloudinary cleanup works, the function returns `nil`.
  Done when: the DB error is always returned.

- [x] **2.3 Product list: pagination and filters**
  Hard-coded `Limit(10)`, `page`/`limit` ignored, `max-price` tag should be `max_price`, inactive products returned.
  Done when: `page`/`limit` work (with a max limit), `max_price` works, inactive products hidden from public listing.

- [x] **2.4 Correct status codes**
  Unknown product returns 401 (`ValidateProductId`) or 500 "User Not Found" (`GetAProduct`); duplicate product returns 401.
  Done when: not found → 404, duplicate → 409.

- [x] **2.5 Cart scoped to a store**
  A cart can mix products from different stores.
  Done when: each cart belongs to one store and rejects other stores' products.

- [x] **2.6 Cart cookie works across domains**
  `c.SetCookie(... secure=false ...)` with no `SameSite`.
  Done when: cookie is `Secure` + `SameSite=None` in production. Set `APP_ENV=production` in production; anything else gives `SameSite=Lax` for http://localhost.

- [x] **2.7 Validation messages**
  `ValidationErrors` says "Password length…" for every `min` tag, has no message for `gte`/`len`, and panics if the error isn't `validator.ValidationErrors`. Currency uses `gte=3` instead of `len=3`.

- [ ] **2.8 Checkout lock 409 on a brand-new cart**
  In the sandbox on 23 Oct, the first `POST /storefront/checkout` on a fresh cart (new `cart_session` cookie, one item just added) returned 409 "Checkout is already in progress for this cart"; the same steps worked straight after on another new cart. The lock (`pkg/services/order/order.go:77`, `checkout-lock:<session>:<store>`, 30 s) should never clash for a new session, so find out why (a repeated request through ngrok? a Redis `SET NX` quirk?).
  Done when: the cause is known and fixed, or shown to be harmless.

---

## 3. Database and tests

- [x] **3.1 Unique constraints and indexes** (migration `000007`)
  - `admins(email)`, `admins(username)` unique
  - `customers(store_id, email)` unique
  - `products(public_id)` unique, index on `products(store_id)`
  - index on `images(product_id)`
  - `stores(admin_id, store_name)` unique
  - also `products(store_id, name)` unique; emails, usernames and names compared case-insensitively. Duplicate-key errors return 409.

- [ ] **3.2 DB-backed tests for services**
  Done so far: cart, product (update, list, add duplicates), ownership, image saving, duplicate/race checks for admin, store and customer signup, migration down/up. Still missing: admin signin, store create happy path, customer signin/profile, product delete and image upload. Run them against a real Postgres (Docker locally or CI service container).

- [ ] **3.3 Controller/route tests**
  HTTP-level tests for each route: auth required, wrong owner → 403, bad payload → 400.
  Started: `pkg/router/http_helpers_test.go` runs the full router on Postgres. Covered: store, store key, storefront, cart, CORS, orders, payments/webhook. Still missing: admin signin, product routes (update/delete/images), customer signin/profile.

- [x] **3.4 Keep Swagger in sync**
  `doc/swagger.json` is hand-written OpenAPI 3.0 (not generated), served at `/swagger/index.html`. Rewritten to match the code in Oct 2026. `pkg/router/swagger_test.go` fails if a route is added or removed without updating it, so update the file with every API change.

---

## 4. Store features needed to sell

- [x] **4.1 Store management** — `GET /store/stores` (list mine), `GET /store/:id`, `PUT /store/:id` (owner only; never returns the API key).
- [x] **4.2 Public storefront API** — `GET /storefront/store`, `GET /storefront/products` (active only, paging/filters/sort), `GET /storefront/products/:id`.
- [x] **4.3 API key middleware** — `X-Store-Key` header, looked up by hash. Also used by `/cart` (replaces `X-Store-Id`). The key is **publishable** (visible in storefront pages): never let it unlock seller actions.
- [x] **4.4 CORS** — `CORS_ALLOWED_ORIGINS` (comma-separated exact origins or `https://*.huddle.ng` wildcards), credentials allowed, other origins' preflights get 403. Storefronts must use `credentials: "include"`. Cart paths have no trailing slash (`/cart`) because browsers don't follow redirects during CORS.
- [x] **4.4a Regenerate store key** — `POST /store/:id/key` (owner only) issues a new key and the old one stops working immediately.
- [x] **4.4b Store domains in the database** — keeping every seller's custom domain in `CORS_ALLOWED_ORIGINS` won't scale to 200 stores. Migration `000013` `store_domains` (bare lowercase hostname, unique across stores, deleted with the store). Seller routes `GET/POST /store/:id/domains`, `DELETE /store/:id/domains/:domain` (owner only): input like `https://ChiomasBags.com/` is saved as `chiomasbags.com`; paths, ports, IPs, `localhost` and Huddle subdomains (under a `CORS_ALLOWED_ORIGINS` wildcard) get 400; another store's domain or a 6th domain gets 409; re-adding returns 200. CORS allows `CORS_ALLOWED_ORIGINS` plus `https://<store domain>` (https only, `www.` is separate), via an in-memory cache (`pkg/storedomain`, 1 min, cleared at once on this server when a domain is added/removed). Because a preflight carries no store key, any store's domain passes CORS, so `StoreKeyMiddleware` then requires the page's domain to belong to the key's store (403 otherwise); env-listed origins, the API's own origin (Swagger UI) and requests without `Origin` may use any key. Domain ownership is not verified yet (6.8).
- [x] **4.5 Orders** — `orders` + `order_items` tables. `POST /storefront/checkout` (guest or signed-in customer; copies prices, holds stock 30 min), `GET /storefront/orders/:id`, `GET /customer/orders`, seller `GET/PATCH /store/:id/orders[/:orderId]`. Unpaid orders expire every minute and their stock goes back on sale.
- [x] **4.6 Bachs payments** ([docs.bachs.io](https://docs.bachs.io/introduction)) — `POST /storefront/orders/:id/pay` opens a Bachs checkout charged to the store's Bachs sub-account (direct charge, `X-Account-Id`); `POST /webhooks/bachs` verifies the HMAC signature, handles each event once and marks the order paid only when amount, currency and account match. Late payments revive the order if stock allows, otherwise they are flagged `needs_refund`.
  - Decided: **sellers collect** (each store's money lands in its own Bachs balance). Platform fee supported via `BACHS_PLATFORM_FEE_PERCENT`, 0 for now.
  - Env: `BACHS_API_KEY`, `BACHS_WEBHOOK_SECRET`, `BACHS_BASE_URL` (default sandbox). Register the webhook URL twice, for Account events and Connected account events; `BACHS_WEBHOOK_SECRET` takes both secrets comma-separated.
  - Outside production a store without a sub-account is charged to the platform account (for sandbox testing); in production it gets 409.
- [x] **4.6b Seller Bachs onboarding** — `POST /store/:id/payments/onboarding` (owner only) creates the store's Bachs account if needed (idempotency key = store id, so concurrent calls share one account; requests `ngn_card_collection`, `bank_transfer`, `payouts`) and returns a single-use hosted onboarding link (`update` link once the store is ready). `GET /store/:id/payments` reads the account from Bachs and returns `connected`, `ready`, capability statuses (with Bachs' message) and outstanding details; falls back to the last known statuses if Bachs is down. `capability.updated` webhooks update `bachs_capabilities` (an older event can't overwrite a newer status). **Ready** = `ngn_card_collection` or `bank_transfer` active; in production, paying needs a ready store (409 otherwise).
- [x] **4.6c Payment follow-ups for sellers** — seller order views (`GET/PATCH /store/:id/orders[/:orderId]`) include a `payment` block (the payment that took money, else the latest checkout; status and latest refund status); shoppers don't see it. `GET /store/:id/payments/issues` lists `needs_refund` / `review` payments. `POST /store/:id/payments/:paymentId/refund` refunds in full or `amount` of it through `POST /v1/refunds` with `X-Account-Id` (Bachs takes one refund per charge). Bachs keeps its fee (2% in sandbox), so a full refund of a seller's only sale fails: Huddle answers 409 with their available balance so they can refund up to it. The refund row is saved first and its id is the reference and idempotency key, so a retry after a timeout re-sends the same refund. `refund.created/paid/failed` webhooks settle it (a final status is never overwritten). A paid refund marks the payment `refunded`, and cancels the order + restores stock if that payment paid the order and it isn't fulfilled, unless the seller sent `keep_order: true`. Payment webhooks now save the Bachs `charge_id` refunds need. Full refund tried in the sandbox on 23 Oct ✔.
- [x] **4.7 Seller order notifications** — notify the seller on each paid order by email (WhatsApp alerts are part of 6.5).
  - Email on `feat/order-notifications` (pushed, PR open), seen end to end in the sandbox on 23 Oct ✔. Over SMTP: `SMTP_HOST`, `SMTP_PORT` (default 587; 465 = TLS from the start, others STARTTLS when offered), `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM` (e.g. `Huddle <orders@huddle.ng>`). Without `SMTP_HOST`/`SMTP_FROM` the API still starts and emails stay queued.
  - When a Bachs payment marks an order paid (including a late payment that revives it), migration `000012` `notifications` gets a row in the same transaction (one per order). A worker sends due rows every minute to the store owner's current email (items, total, customer contact, delivery/pickup, note) and retries failures after 1m, 4m, 16m… up to 6h, giving up after 8 attempts (`failed`, `last_error` kept).
  - No email when the seller marks an order paid themselves, or for payments flagged `needs_refund`/`review` (decided 21 Oct; those are listed at `GET /store/:id/payments/issues`).
---

## 5. Production readiness (enough for 200 SMEs on one server)

- [x] **5.1 Health check** — `GET /healthz` (no auth) pings Postgres and Redis within 2 s: 200 `{postgres: ok, redis: ok}`, else 503 with the one that is `down`. Error details only go to the server log; `Cache-Control: no-store`; successful checks are left out of the request log so monitors don't flood it.
- [x] **5.2 Graceful shutdown + server timeouts** — `main.go` serves through an `http.Server` (`server.go`): read-header 10 s, read 60 s (image uploads on slow data), write 60 s (Bachs onboarding takes up to 40 s), idle 120 s. On Ctrl+C or SIGTERM it stops taking connections and gives requests in progress up to 45 s, then waits (same limit) for the expiry and email workers to finish their current run (an email being sent is not cut off), then closes Redis and Postgres. **Docker/hosts must wait longer than 45 s before killing it** (Docker's default is 10 s: set `stop_grace_period: 50s` in 5.6).
- [x] **5.3 Config** — `pkg/config` checks every setting at startup; the API refuses to start with a list of what to fix (setting names only, never values) and logs warnings for risky gaps. `PORT` (default 8080); `APP_ENV=production` turns on Gin release mode, the production cart cookie (2.6) and production payment rules.
  - **Stops startup:** `APP_ENV` other than `production`/`development` (a typo like `prod` would silently skip production behaviour); bad `PORT`; no `SECRET_KEY`, or under 32 characters in production; no `DB_HOST`/`DB_USER`/`DB_NAME`; bad `DB_SSLMODE`; `ORDER_HOLD_MINUTES` not a whole number above 0; `BACHS_PLATFORM_FEE_PERCENT` not 0 to below 100; bad `TRUSTED_PROXIES`; in production, no `BACHS_API_KEY`/`BACHS_WEBHOOK_SECRET`.
  - **Warns in production:** `DB_SSLMODE` unset/`disable`, no `TRUSTED_PROXIES`, no `CORS_ALLOWED_ORIGINS`, no SMTP, no `CLOUDINARY_URL`.
  - **New settings:** `DB_SSLMODE` (default `disable`; managed Postgres usually needs `require`) and `TRUSTED_PROXIES` (IPs/CIDRs of the load balancer). **Security fix:** Gin believed `X-Forwarded-For` from anyone, so a client could fake its IP and dodge the sign-in rate limiter; now only `TRUSTED_PROXIES` may set it, otherwise the connecting address is used. Set it in production behind a proxy, or every client shares the proxy's IP for rate limiting.
  - The local `.env` passes the check (development, port 8080, no problems).
- [ ] **5.4 Cloudinary client created once** — not per upload (`internal/cloudinary/client.go`).
- [ ] **5.5 Rate limiter in Redis** — so it works across more than one server.
- [ ] **5.6 Dockerfile + docker-compose** — API, Postgres, Redis. Give the API `stop_grace_period: 50s` (see 5.2).
- [ ] **5.7 CI** — `go vet`, `go test`, build on every push.
- [ ] **5.8 Deploy + backups** — managed Postgres with daily backups, logs collected.
- [ ] **5.9 Repo cleanup** — remove committed `main.exe`, add it to `.gitignore`.

---

## 6. Simple for everyone (WhatsApp + AI)

- [ ] **6.1 Phone number + OTP login** — no passwords for sellers.
- [ ] **6.2 Customer website (storefront template) + theme settings** — one fast, mobile-first template, built and tested once by Huddle and shared by every store (light enough for cheap Android phones on mobile data). Pages, all on APIs that already exist: shop home/product list (`GET /storefront/store`, `/storefront/products`), product page, cart (`/cart`), checkout (`POST /storefront/checkout`), pay (`POST /storefront/orders/:id/pay` → Bachs), order status/thank-you (`GET /storefront/orders/:id`), optional customer sign-in and order history (`/customer/*`).
  - API work: per-store **theme settings** (logo, colours, banner, fonts, which sections to show, shop description), a seller route to save them, returned by `GET /storefront/store`.
  - To decide: the tech stack, where it is hosted and how each store gets its address.
- [ ] **6.3 AI sets each store's theme and content** — sellers never touch a theme editor: from chat or photos ("make it red and gold, like Ankara", a logo), the agent updates the store's theme settings and writes product descriptions. It edits **settings and content only, never code**, so it can't break checkout and every store keeps running the one tested template. The seller approves in the preview (6.7). Decided 23 Oct over having AI write a separate website per store (200 codebases to fix, risky for payments, slow and costly to regenerate).
- [ ] **6.4 Photo → product** — seller sends a photo and price; the agent creates the product.
- [ ] **6.5 WhatsApp bot** — set up store, add products, get order alerts, all in chat.
- [ ] **6.6 Voice notes in Pidgin, Yoruba, Igbo, Hausa** — the agent understands them.
- [ ] **6.7 Preview before publish** — seller approves the store before it goes live.
- [ ] **6.8 Verify custom domain ownership** — before opening to many sellers: today any seller can claim any domain first (4.4b), which can't harm the real owner's site but can block them from adding it. Seller adds a `huddle-verify=<token>` DNS TXT record; only verified domains pass CORS. Until then, support removes a squatted domain from `store_domains`. When Huddle hosts storefronts (6.2), custom domains also need automatic HTTPS. Domain selling was considered and dropped (23 Oct).
- [ ] **6.9 Developer docs** — later, only once a developer asks. Developers can already build their own storefront on the same API (`/storefront/*`, `/cart`, checkout, pay) with the store key, and connect their domain (4.4b). Still missing: getting-started docs beyond Swagger, and a test-mode store.

---

## Future updates

Not in the current plan; pick up once the basics are live and there is demand.

- [ ] **Music downloads (MP3)** — for musicians selling their own music directly. Moved out of the current plan on 23 Oct: musicians sell merch (normal products) as soon as the API is deployed and the customer website (6.2) is up; build this once they are selling and there is clear demand. Needs 6.2 (plays previews, serves downloads). Decided so far:
  - **Product type `digital`:** no stock limit (optional limited edition), no delivery address. Checkout requires the shopper's email when the cart has a digital item.
  - **Files:** **MP3 only** for now (check the content, not just the name; about 20 MB per track; recommend 320 kbps). An album is several tracks. The artist uploads a separate short **preview clip** (auto-cutting previews needs ffmpeg: later). WAV/FLAC maybe later.
  - **Storage: AWS S3** (private bucket, nearest region `af-south-1` Cape Town). Full tracks are never public; previews can be. Product images stay on Cloudinary; move them only if there's a reason later.
  - **Delivery:** once Bachs confirms payment, the order page shows download links and the shopper gets an email with them (reuse the 4.7 email worker). Links are S3 presigned URLs issued per paid order, valid a few days and a few downloads (e.g. 7 days, 5 downloads).
  - **Piracy:** can't be fully prevented (anything playable can be recorded; DRM is not worth it). Make sharing traceable and buying easy: stamp each copy's ID3 tags with the buyer and order ("Licensed to …, order ord_…"); expiring links; full tracks never public. An inaudible audio watermark only if ever needed.
  - **Rights:** sellers may only sell music they own or have rights to (distributor contracts may limit direct sales). Add this to the terms, plus a takedown process that can trace a leaked copy to its order.


---

## Done

- [x] Re-evaluated codebase (Oct 2026)
- [x] First tests: utility, middleware, response, cart, image validation (`go test ./pkg/...` passes)
- [x] Section 1 security fixes, with Postgres-backed tests for cart and ownership
