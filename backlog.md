# Huddle Backlog

Goal: a secure, reliable store platform that at least 200 Nigerian SMEs can use, simple enough that a market woman can run her shop from WhatsApp.

Pick items top to bottom. Each item is done when its "Done when" line is true and `go test ./...` passes.

Database tests use your local Postgres: a separate `huddle_test` database (never the dev `huddle` one), one throwaway schema per test, built from `db/migrations`. Set `TEST_DATABASE_URL` to point elsewhere. If Postgres is unreachable, those tests skip.

## Where we are (updated 21 Oct 2026)

- **Branch:** `feat/bachs-payment-followups` has 4.6c (incl. partial refunds), committed (`5721bf0`) and pushed to `origin` on 21 Oct. Local `main` has 4.6b. Next:
  1. ~~Run `go test ./...` and `go vet ./...`~~ both pass with partial refunds (16 Oct; `go test` again 21 Oct).
  2. Finish the sandbox refund test (below). Not started yet: on 20 Oct Claude Code tried to start the API and ngrok itself, but the machine ran out of memory (even small commands timed out) and everything was stopped. Start both in your own terminals, then give Claude the ngrok address.
  3. Merge into `main`, then start **4.7**. `origin/main` has 2 README commits (`08b80b6`, `62de7b2`) that local `main` doesn't, so merge `origin/main` in before pushing `main`.
- **Sandbox refund test, in progress.** Test store `str-e5564c2ac9824cbdaeb4f0de579d0357` (seller `sandbox1792093898`, password `secret123`, Bachs account `acct_EPpm2BjrPWW263rN`). Its order `ord_fe532f63baea4c23a4da1cf447c25dde` is paid (₦15,000, charge id saved). Bachs kept a 2% fee, so the seller's balance is ₦14,700 and a full refund returns 409 with that balance (checked against the real sandbox ✔). Still to prove: the successful refund. Place and pay one more ₦15,000 order for this store (the earlier second order expired unpaid), so the balance is ₦29,400, then refund `ord_fe53…`'s payment in full and check that `refund.paid` arrives, the payment turns `refunded`, the order `cancelled` and the stock comes back. To get a store key for a new order, call `POST /store/{id}/key` as the seller.
- **Running locally:** start the API (`go run .`, port 8080) and `ngrok http 8080` in **your own terminals**. Claude Code stops background processes when the machine is low on memory (it happened 3 times). The ngrok address has stayed `https://outward-finally-elite.ngrok-free.dev`; if it changes, update both Bachs webhook endpoints. Shoppers pay on the Bachs checkout page by hand, because the Chrome extension has no access to `sandbox-checkout.bachs.io`. Sandbox card: any number except Bachs' decline cards (`4000 0000 0000 0002/9995/0069/0119`), or bank transfer with the page's test controls.
- **Local dev DB** (`huddle`) is at migration **11**. Apply new migrations with `~/go/bin/migrate -path db/migrations -database "postgres://USER:PASS@localhost:5432/huddle?sslmode=disable" up` (values from `.env`; `.env` uses `KEY = value` with spaces). **Never roll back and re-apply a migration on the dev DB to change it once it holds data**: that wiped saved charge ids on 15 Oct (recovered from ngrok's request log). Add a new migration instead, unless the migration is uncommitted and the data doesn't matter.
- **Bachs:** sandbox keys are in `.env`: `BACHS_API_KEY` (Full Access sandbox key, named `huddle-sandbox`; for live use only Connected Accounts, Payments and Refunds: Write), `BACHS_WEBHOOK_SECRET` (optional `BACHS_BASE_URL`, `BACHS_PLATFORM_FEE_PERCENT`). Bachs webhook endpoints listen to one source each, so the same URL is registered twice (**Connected account events** = `huddle-sellers`, **Account events** = `huddle-platform`) and `BACHS_WEBHOOK_SECRET = secret1,secret2`. Events selected: Account Updated, Capability Updated, Checkout Completed, Checkout Expired, Collection Succeeded, Refund Created/Failed/Paid. Tested in the sandbox on 15 Oct 2026: onboarding created a real sub-account (all capabilities active at once in sandbox, details still "outstanding"), the store showed ready, a ₦15,000 order was paid on the hosted checkout, and `checkout.completed` + `collection.succeeded` arrived through ngrok and marked it paid. `capability.updated` has not been seen yet (sandbox activates capabilities at creation).
- **Bachs API notes:** calling it from Python needs a `User-Agent` header (Cloudflare blocks urllib's default with `error code: 1010`). An account's balance is `GET /v1/balances` with `X-Account-Id`. The charge id is `data.charge.id` on `checkout.completed` and `data.charge_id` on `collection.succeeded`. Refunds need a balance that covers them; Bachs' fee is not returned.
- **Decisions made:** guest checkout + optional customer account; delivery details = name, phone, pickup/delivery (+ address/city/state for delivery), optional email and note; stock hold 30 min; sellers can mark orders paid themselves (transfer/cash); **sellers collect** payments in their own Bachs sub-account (direct charges); platform fee 0 for now.

## How we work

- One feature branch per item, fast-forward merge into `main` when asked. Push to `origin` (GitHub) only when asked.
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
- [ ] **4.4b Store domains in the database** — keeping every seller's custom domain in `CORS_ALLOWED_ORIGINS` won't scale to 200 stores. Store each store's domain(s) and check origins against them (subdomains of the Huddle domain can stay a wildcard).
- [x] **4.5 Orders** — `orders` + `order_items` tables. `POST /storefront/checkout` (guest or signed-in customer; copies prices, holds stock 30 min), `GET /storefront/orders/:id`, `GET /customer/orders`, seller `GET/PATCH /store/:id/orders[/:orderId]`. Unpaid orders expire every minute and their stock goes back on sale.
- [x] **4.6 Bachs payments** ([docs.bachs.io](https://docs.bachs.io/introduction)) — `POST /storefront/orders/:id/pay` opens a Bachs checkout charged to the store's Bachs sub-account (direct charge, `X-Account-Id`); `POST /webhooks/bachs` verifies the HMAC signature, handles each event once and marks the order paid only when amount, currency and account match. Late payments revive the order if stock allows, otherwise they are flagged `needs_refund`.
  - Decided: **sellers collect** (each store's money lands in its own Bachs balance). Platform fee supported via `BACHS_PLATFORM_FEE_PERCENT`, 0 for now.
  - Env: `BACHS_API_KEY`, `BACHS_WEBHOOK_SECRET`, `BACHS_BASE_URL` (default sandbox). Register the webhook URL twice, for Account events and Connected account events; `BACHS_WEBHOOK_SECRET` takes both secrets comma-separated.
  - Outside production a store without a sub-account is charged to the platform account (for sandbox testing); in production it gets 409.
- [x] **4.6b Seller Bachs onboarding** — `POST /store/:id/payments/onboarding` (owner only) creates the store's Bachs account if needed (idempotency key = store id, so concurrent calls share one account; requests `ngn_card_collection`, `bank_transfer`, `payouts`) and returns a single-use hosted onboarding link (`update` link once the store is ready). `GET /store/:id/payments` reads the account from Bachs and returns `connected`, `ready`, capability statuses (with Bachs' message) and outstanding details; falls back to the last known statuses if Bachs is down. `capability.updated` webhooks update `bachs_capabilities` (an older event can't overwrite a newer status). **Ready** = `ngn_card_collection` or `bank_transfer` active; in production, paying needs a ready store (409 otherwise).
- [x] **4.6c Payment follow-ups for sellers** — seller order views (`GET/PATCH /store/:id/orders[/:orderId]`) include a `payment` block (the payment that took money, else the latest checkout; status and latest refund status); shoppers don't see it. `GET /store/:id/payments/issues` lists `needs_refund` / `review` payments. `POST /store/:id/payments/:paymentId/refund` refunds in full or `amount` of it through `POST /v1/refunds` with `X-Account-Id` (Bachs takes one refund per charge). Bachs keeps its fee (2% in sandbox), so a full refund of a seller's only sale fails: Huddle answers 409 with their available balance so they can refund up to it. The refund row is saved first and its id is the reference and idempotency key, so a retry after a timeout re-sends the same refund. `refund.created/paid/failed` webhooks settle it (a final status is never overwritten). A paid refund marks the payment `refunded`, and cancels the order + restores stock if that payment paid the order and it isn't fulfilled, unless the seller sent `keep_order: true`. Payment webhooks now save the Bachs `charge_id` refunds need. Not tried in the sandbox yet.
- [ ] **4.7 Seller order notifications** — notify the seller on each paid order (email first, WhatsApp later).

---

## 5. Production readiness (enough for 200 SMEs on one server)

- [ ] **5.1 Health check** — `GET /healthz` checking Postgres and Redis.
- [ ] **5.2 Graceful shutdown + server timeouts** — replace `r.Run` with `http.Server`.
- [ ] **5.3 Config** — port from env, Gin release mode in production. Set `APP_ENV=production` (cart cookie, see 2.6), `CORS_ALLOWED_ORIGINS` (see 4.4) and optionally `ORDER_HOLD_MINUTES` (default 30, see 4.5).
- [ ] **5.4 Cloudinary client created once** — not per upload (`internal/cloudinary/client.go`).
- [ ] **5.5 Rate limiter in Redis** — so it works across more than one server.
- [ ] **5.6 Dockerfile + docker-compose** — API, Postgres, Redis.
- [ ] **5.7 CI** — `go vet`, `go test`, build on every push.
- [ ] **5.8 Deploy + backups** — managed Postgres with daily backups, logs collected.
- [ ] **5.9 Repo cleanup** — remove committed `main.exe`, add it to `.gitignore`.

---

## 6. Simple for everyone (WhatsApp + AI)

- [ ] **6.1 Phone number + OTP login** — no passwords for sellers.
- [ ] **6.2 Storefront template** — one fast, mobile-first template that reads store + theme.
- [ ] **6.3 AI agent reads the Huddle API** — routes, models, Swagger and `context.md` (allowlisted files only, never `.env`) to build and connect a storefront from the template.
- [ ] **6.4 Photo → product** — seller sends a photo and price; the agent creates the product.
- [ ] **6.5 WhatsApp bot** — set up store, add products, get order alerts, all in chat.
- [ ] **6.6 Voice notes in Pidgin, Yoruba, Igbo, Hausa** — the agent understands them.
- [ ] **6.7 Preview before publish** — seller approves the store before it goes live.

---

## Done

- [x] Re-evaluated codebase (Oct 2026)
- [x] First tests: utility, middleware, response, cart, image validation (`go test ./pkg/...` passes)
- [x] Section 1 security fixes, with Postgres-backed tests for cart and ownership
