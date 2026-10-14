# Huddle Backlog

Goal: a secure, reliable store platform that at least 200 Nigerian SMEs can use, simple enough that a market woman can run her shop from WhatsApp.

Pick items top to bottom. Each item is done when its "Done when" line is true and `go test ./...` passes.

Database tests use your local Postgres: a separate `huddle_test` database (never the dev `huddle` one), one throwaway schema per test, built from `db/migrations`. Set `TEST_DATABASE_URL` to point elsewhere. If Postgres is unreachable, those tests skip.

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

- [ ] **2.6 Cart cookie works across domains**
  `c.SetCookie(... secure=false ...)` with no `SameSite`.
  Done when: cookie is `Secure` + `SameSite=None` in production.

- [ ] **2.7 Validation messages**
  `ValidationErrors` says "Password length…" for every `min` tag, has no message for `gte`/`len`, and panics if the error isn't `validator.ValidationErrors`. Currency uses `gte=3` instead of `len=3`.

---

## 3. Database and tests

- [ ] **3.1 Unique constraints and indexes** (new migration)
  - `admins(email)`, `admins(username)` unique
  - `customers(store_id, email)` unique
  - `products(public_id)` unique, index on `products(store_id)`
  - index on `images(product_id)`
  - `stores(admin_id, store_name)` unique

- [ ] **3.2 DB-backed tests for services**
  Admin, store, product and customer services have no tests. Run them against a real Postgres (Docker locally or CI service container).

- [ ] **3.3 Controller/route tests**
  HTTP-level tests for each route: auth required, wrong owner → 403, bad payload → 400.

- [ ] **3.4 Keep Swagger in sync**
  `doc/swagger.json` is missing `/cart/items/{id}`, and `POST /cart` now takes only `product_id` + `quantity`. All `/cart` routes need an `X-Store-Id` header. `GET /product/products` returns `{items, page, limit, total}`. Customer profile needs a customer token. Regenerate with `swag init` on each change (or check in CI).

---

## 4. Store features needed to sell

- [ ] **4.1 Store management** — `GET /store/:id`, `PUT /store/:id`, list an admin's stores.
- [ ] **4.2 Public storefront API** — `/storefront/*` routes authenticated with the store API key (no admin token): list products, product detail, store info. Reuse `helper.GetAllProductQueryChain` with `Active` forced to `true` so hidden products never show.
- [ ] **4.3 API key middleware** — validate the hashed key from `stores.api_key`.
- [ ] **4.4 CORS** — allow storefront domains to call the API.
- [ ] **4.5 Orders** — `orders` + `order_items` tables, `POST /orders/checkout` (snapshot prices, reserve stock), `GET /orders`, `GET /orders/:id`, order status.
- [ ] **4.6 Paystack payments** — initialize payment, verify, webhook with signature check, mark order paid.
- [ ] **4.7 Seller order notifications** — notify the seller on each paid order (email first, WhatsApp later).

---

## 5. Production readiness (enough for 200 SMEs on one server)

- [ ] **5.1 Health check** — `GET /healthz` checking Postgres and Redis.
- [ ] **5.2 Graceful shutdown + server timeouts** — replace `r.Run` with `http.Server`.
- [ ] **5.3 Config** — port from env, Gin release mode in production.
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
