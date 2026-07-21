# Huddle E-Commerce API — Context & Roadmap

## 1. Project Purpose & Vision

**Huddle** is a multi-tenant e-commerce backend platform built with **Go (Golang)**. It is designed to allow administrators to create and manage independent online stores while providing endpoints for store management, catalog administration, product media management, customer accounts, and shopping cart operations.

---

## 2. Technology Stack & Key Packages

| Category | Technology / Package | Description & Usage |
| :--- | :--- | :--- |
| **Language & Version** | Go `1.25.1` | Core language runtime |
| **Web Framework** | `github.com/gin-gonic/gin` `v1.11.0` | High-performance HTTP web framework & routing |
| **Database ORM** | `gorm.io/gorm` `v1.31.1`<br>`gorm.io/driver/postgres` `v1.6.0` | PostgreSQL Object Relational Mapper |
| **Cache & In-Memory Store** | `github.com/redis/go-redis/v9` `v9.17.1` | Caching public-to-internal ID mappings & session tracking |
| **Media Cloud Storage** | `github.com/cloudinary/cloudinary-go/v2` `v2.14.0` | Cloudinary integration for product image uploads & deletion |
| **Authentication & Security**| `github.com/golang-jwt/jwt/v5` `v5.3.0`<br>`golang.org/x/crypto` `v0.45.0` | JWT token generation/validation & Bcrypt password hashing |
| **Validation** | `github.com/go-playground/validator/v10` `v10.28.0` | Request payload and struct validation |
| **Utilities** | `github.com/google/uuid` `v1.6.0`<br>`github.com/shopspring/decimal` `v1.4.0`<br>`github.com/joho/godotenv` `v1.5.1` | UUID generation, arbitrary precision financial arithmetic, `.env` file loading |
| **API Documentation** | `github.com/swaggo/gin-swagger` `v1.6.1`<br>`github.com/swaggo/files` `v1.0.1` | Swagger UI rendering served at `/swagger/index.html` |

---

## 3. Project Architecture & Directory Structure

The project follows a modular, layered architecture separating routes, controllers, services, database models, and helpers:

```
huddle/
├── db/
│   ├── connection/       # Database & Cache connection initializers (PostgreSQL, Redis)
│   └── migrations/       # SQL up/down database schema migration files
├── doc/                  # Swagger documentation assets (swagger.go, swagger.json)
├── internal/
│   └── cloudinary/       # Cloudinary client SDK initialization & upload/delete helpers
├── pkg/
│   ├── controller/       # Request handlers (Admin, Store, Product, Customer, Cart)
│   ├── helper/           # Domain query builders, image validators, ownership checks
│   ├── middleware/       # Auth JWT middleware, Store Context header, Rate Limiter
│   ├── model/            # GORM models & DTO struct definitions
│   ├── response/         # Standardized JSON response format helpers
│   ├── router/           # Gin endpoint group definitions
│   ├── services/         # Business logic layer (DB queries, Redis cache, Cloudinary)
│   ├── test/             # Test suite location (to be populated)
│   └── utility/          # Shared functions (JWT, Password Hashing, UUID, API Key generator)
├── go.mod / go.sum       # Go module dependencies
└── main.go               # Application entry point
```

---

## 4. Summary of API Endpoints

### 🛡️ Admin Management (`/admin`)
| Method | Endpoint | Auth | Description | Status |
| :--- | :--- | :--- | :--- | :--- |
| `POST` | `/admin/signup` | Public | Register a new platform administrator; returns JWT token. | ✅ Completed |
| `POST` | `/admin/signin` | Public | Authenticate administrator; returns JWT token. | ✅ Completed |

### 🏬 Store Management (`/store`)
| Method | Endpoint | Auth | Description | Status |
| :--- | :--- | :--- | :--- | :--- |
| `POST` | `/store/create` | Bearer JWT | Create a new tenant store; generates unique `store_id` & API Key. | ✅ Completed |

### 📦 Product & Image Management (`/product`)
| Method | Endpoint | Auth | Description | Status |
| :--- | :--- | :--- | :--- | :--- |
| `POST` | `/product/` | Bearer JWT + `X-Store-Id` | Create a product under a specific store. | ✅ Completed |
| `GET` | `/product/products` | Bearer JWT | List products for a store (supports `store_id`, `min_price`, `max_price`). | ✅ Completed |
| `GET` | `/product/:id` | Bearer JWT | Retrieve a product by `public_id` alongside associated image gallery. | ✅ Completed |
| `PUT` | `/product/:id` | Bearer JWT | Update product details (name, description, price, stock, active status). | ✅ Completed |
| `DELETE` | `/product/:id` | Bearer JWT | Delete product, purge images from Cloudinary, and clear Redis cache. | ✅ Completed |
| `POST` | `/product/:id/images` | Bearer JWT | Upload up to 5 images (max 5MB each) to Cloudinary for a product. | ✅ Completed |

### 👤 Customer Management (`/customer`)
| Method | Endpoint | Auth | Description | Status |
| :--- | :--- | :--- | :--- | :--- |
| `POST` | `/customer/signup` | Public | Register a new customer under a tenant store; returns JWT token. | ✅ Completed |
| `POST` | `/customer/signin` | Public | Authenticate store customer against password; returns JWT token. | ✅ Completed |
| `GET` | `/customer/profile` | Bearer JWT | Fetch authenticated customer profile. | ✅ Completed |

### 🛒 Shopping Cart (`/cart`)
| Method | Endpoint | Auth / Cookie | Description | Status |
| :--- | :--- | :--- | :--- | :--- |
| `POST` | `/cart` | Session Cookie | Add item to shopping cart (persisted in Redis with 7-day TTL). | ✅ Completed |

### 📄 Documentation & System
| Method | Endpoint | Auth | Description | Status |
| :--- | :--- | :--- | :--- | :--- |
| `GET` | `/` | Public | API welcome message. | ✅ Completed |
| `GET` | `/swagger/*any` | Public | Swagger UI API documentation. | ✅ Completed |

---

## 5. What Has Been Completed So Far

1. **Core Infrastructure**:
   - Environment setup (`godotenv`), PostgreSQL connection (`GORM`), Redis client connection.
   - Comprehensive database migration files for `admins`, `stores`, `products`, `images`, and `customers`.
   - Global response handling (`pkg/response`) for uniform API responses (`success`, `message`, `data`, `error`).
   - Centralized struct validation using `go-playground/validator`.

2. **Authentication & Authorization System**:
   - Admin signup and signin with Bcrypt password hashing.
   - JWT generation & verification middleware (`AuthMiddleware`).
   - Header-based store tenant scoping (`StoreContextMiddleware` reading `X-Store-Id`).

3. **Store Creation & Ownership**:
   - Admin store creation generating unique public IDs (`str-<uuid>`) and secure API keys.
   - Store ownership verification logic (`VerifyOwnerShip`).

4. **Product Catalog & Media Upload Pipeline**:
   - Product creation with Redis caching for public ID to internal UUID lookup (`30-minute TTL`).
   - Multi-file image upload endpoint: validates image files (1-5 files, <=5MB, MIME/image format check), uploads to Cloudinary, stores image records in PostgreSQL, with automatic rollback/deletion from Cloudinary if DB insertion fails.
   - Product filtering by store ID and price bounds (`min_price`, `max_price`).
   - Cascade deletion of product records, Cloudinary assets, and Redis cache keys.

---

## 7. Senior Engineer Refactoring & Quality Upgrades

1. **Database & Connection Pooling**:
   - Upgraded [postgres.go](file:///C:/Users/USER/Desktop/HNG/huddle/db/connection/postgres.go) with GORM connection pool limits (`SetMaxOpenConns(25)`, `SetMaxIdleConns(10)`, `SetConnMaxLifetime(15m)`, `SetConnMaxIdleTime(5m)`).
   - Upgraded [redis.go](file:///C:/Users/USER/Desktop/HNG/huddle/db/connection/redis.go) to read `REDIS_ADDR` and `REDIS_PASSWORD` dynamically from environment variables with pool sizing.

2. **Thread Safety & Rate Limiting**:
   - Replaced unsafe global map rate limiter in [ratelimiter.go](file:///C:/Users/USER/Desktop/HNG/huddle/pkg/middleware/ratelimiter.go) with a thread-safe mutex-protected rate limiter and non-blocking worker thread cleanup.

3. **Standardized Response Envelope & Security**:
   - Upgraded [response.go](file:///C:/Users/USER/Desktop/HNG/huddle/pkg/response/response.go) with `ResponseEnvelope` struct and standard Go `http.StatusOK` / `http.StatusCreated` constants.
   - Fixed typos (`Succesfull` ➔ `Successful`, `coccured` ➔ `occurred`, `Crendentials` ➔ `Credentials`).
   - Added SHA-256 API Key hashing helper in [api_key.go](file:///C:/Users/USER/Desktop/HNG/huddle/pkg/utility/api_key.go).
   - Fixed context extraction order in [store.go](file:///C:/Users/USER/Desktop/HNG/huddle/pkg/controller/store.go) to avoid nil type assertion panics.

4. **Production Structured Audit Logging System**:
   - Implemented [audit.go](file:///C:/Users/USER/Desktop/HNG/huddle/pkg/audit/audit.go) using Go standard library `log/slog` for structured JSON log events (`app`, `event`, `actor_id`, `target_id`, `status`, `timestamp`, `metadata`).
   - Added HTTP request audit middleware [audit.go](file:///C:/Users/USER/Desktop/HNG/huddle/pkg/middleware/audit.go) tracking method, path, status code, latency (ms), client IP, and User-Agent.
   - Integrated domain event auditing for `ADMIN_SIGNUP`, `ADMIN_SIGNIN`, `STORE_CREATED`, `PRODUCT_CREATED`, `PRODUCT_UPDATED`, `PRODUCT_DELETED`, and `PRODUCT_IMAGES_UPLOADED`.


---

## 8. Where We Are Going (Roadmap & Next Steps)

1. **Complete Shopping Cart Management**:
   - `GET /cart` — Retrieve active cart items stored in Redis via `cart_session` cookie.
   - `PUT /cart/items/:item_id` — Update item quantity in cart.
   - `DELETE /cart/items/:item_id` — Remove an item from cart.
   - `DELETE /cart` — Clear entire shopping cart session.

2. **Orders & Checkout Engine**:
   - Create database migration files for `orders` and `order_items` tables.
   - `POST /orders/checkout` — Lock cart items into a confirmed customer order.
   - `GET /orders` & `GET /orders/:id` — Order history and status tracking.
   - Payment gateway webhook integration (Paystack / Stripe).

3. **Store Tenant Management**:
   - `GET /store/:id` — Fetch tenant store profile.
   - `PUT /store/:id` — Update tenant store configuration.
   - Public Storefront Middleware validating tenant `api_key` for catalog browsing.

4. **Automated Testing Suite**:
   - Add unit & integration test suites under `pkg/test`.

