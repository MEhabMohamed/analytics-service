# Node/TypeScript to Go Mapping Guide (`order-service` → `analytics-service`)

This is the flagship reference document for developers who know `core-service` and `order-service` (Node.js + TypeScript + Express + Knex) and want to understand how every architectural pattern translates into idiomatic **Go** in `analytics-service`.

---

## 1. Core Language Idiom Cheat Sheet

| Concept | Node.js / TypeScript (`order-service`) | Go (`analytics-service`) |
| :--- | :--- | :--- |
| **Types & Data Shapes** | `export interface OrderRow { ... }` | `type OrderRow struct { ... }` (`interface` in Go is ONLY for method sets, never fields) |
| **Classes & Methods** | `export class OrderService { async getOrder(...) {} }` | `type Service struct { ... }` + pointer receiver `func (s *Service) GetOrder(...)` |
| **Visibility (`export` / `private`)** | `export class ...` / `private helper()` | Identifier case: `Uppercase` = exported (`GetOrder`), `lowercase` = package-private (`extractUTCDate`) |
| **Decorators / Metadata** | `@IsString() @Expose()` (runtime decorators) | Struct tags in backticks: `` `json:"ordersCount" bson:"orders_count" validate:"required"` `` |
| **Async & Error Handling** | `async fn(): Promise<T>` + `throw new AppError(...)` | Synchronous goroutines + explicit tuple return `(T, error)` + `if err != nil { return ..., err }` |
| **Request Context** | Mutating `req.user`, `req.correlationId` (`express.d.ts`) | Immutable `context.Context` passed as first parameter (`ctx context.Context`) via `appcontext.WithClaims` |
| **Interfaces** | Explicit: `class RabbitMQClient implements IMessageBroker` | Implicit (duck typing): if `EventIDsRepo` has `MarkSeen(...)`, it satisfies `coreevents.EventDeduper` automatically |
| **Dependency Injection** | `@injectable()` + `tsyringe` `container.resolve(TOKENS.X)` | Explicit constructor functions (`service.New(restRepo, ...)`) wired in `lib/boot/boot.go` |
| **Resource Cleanup** | `try { ... } finally { await cursor.close(); }` | `defer cur.Close(ctx)` placed right below resource creation |
| **Empty Array in JSON** | `const out: DTO[] = [];` → `[]` | `out := make([]DTO, 0, len(rows))` → `[]` (warning: `var out []DTO` is `nil` and serializes to `null`!) |

---

## 2. Layer-by-Layer Side-by-Side Comparison

### 2.1 Environment Configuration (`lib/config/`)

**Node/TS (`order-service/src/lib/config/env.ts` — Zod):**
```ts
const baseSchema = z.object({
    PORT: z.string().default("4000"),
    ACCESS_SECRET: z.string(),
    RABBITMQ_ORDER_EVENTS_BINDINGS: z.string().default("order.#,payment.#"),
});
const parsed = baseSchema.parse(process.env);
```

**Go (`analytics-service/lib/config/env.go` — `caarlos0/env/v11`):**
```go
type Config struct {
    Port                int      `env:"PORT" envDefault:"5000"`
    AccessSecret        string   `env:"ACCESS_SECRET,required"`
    OrderEventsBindings []string `env:"RABBITMQ_ORDER_EVENTS_BINDINGS" envSeparator:"," envDefault:"order.#,payment.#"`
}

func Load() (*Config, error) {
    var cfg Config
    if err := env.Parse(&cfg); err != nil {
        return nil, fmt.Errorf("parse config: %w", err)
    }
    return &cfg, nil
}
```

---

### 2.2 Structured Logging & Correlation IDs (`lib/logger/` & `lib/appcontext/`)

**Node/TS (`order-service/src/lib/logger/logger.ts`):**
```ts
logger.info("order created", { publicId: order.publicId, correlationId: req.correlationId });
```

**Go (`analytics-service/lib/logger/logger.go` — stdlib `log/slog`):**
```go
log := logger.FromContext(r.Context(), baseLog) // automatically attaches correlationId
log.Info("mongo connected", "db", cfg.MongoDB)
```

---

### 2.3 Operational Errors & Error Middleware (`lib/errors/`)

**Node/TS (`order-service/src/lib/error/errorHandler.ts`):**
```ts
// Service throws:
if (from > to) throw InvalidDateRangeError;

// Express 4-arg error middleware catches rejected promises:
export function errorHandler(err: Error, req: Request, res: Response, _next: NextFunction) {
    const appErr = err instanceof AppError ? err : new AppError("Unknown", 500, false);
    return res.status(appErr.statusCode).json({ error: appErr.message });
}
```

**Go (`analytics-service/lib/errors/handler.go`):**
```go
// Service returns error value:
if q.From > q.To {
    return nil, analytics.ErrInvalidDateRange
}

// errors.Wrap converts func(w, r) error into standard http.HandlerFunc:
func Wrap(baseLog *slog.Logger, h HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if err := h(w, r); err != nil {
            RenderError(w, r, baseLog, err)
        }
    }
}
```

---

### 2.4 Auth & RBAC Middleware (`lib/auth/` & `lib/rbac/`)

**Node/TS (`order-service/src/lib/auth/guard.ts`):**
```ts
export function authenticate(req: Request, res: Response, next: NextFunction) {
    const token = req.cookies?.access_token;
    if (!token) throw NotAuthenticated;
    req.user = verifyAccessToken(token);
    next();
}
```

**Go (`analytics-service/lib/auth/middleware.go`):**
```go
func Authenticate(secret string, baseLog *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims, err := VerifyAccessToken(extractToken(r), secret)
            if err != nil {
                apperr.RenderError(w, r, baseLog, apperr.ErrUnauthenticated)
                return
            }
            ctx := appcontext.WithClaims(r.Context(), claims)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

---

### 2.5 DTOs: Validation & Wire Formatting (`app/analytics/dto/`)

**Node/TS (`order.response.dto.ts`):**
```ts
export class RestaurantDayResponseDTO {
    date!: string;
    ordersCount!: number;
    static from(row: RestaurantDayRow): RestaurantDayResponseDTO { ... }
}
```

**Go (`analytics.response.go`):**
```go
type RestaurantDayResponseDTO struct {
    Date          string `json:"date"`
    OrdersCount   int64  `json:"ordersCount"`
    RevenueMinor  int64  `json:"revenueMinor"`
    Currency      string `json:"currency"`
    AvgOrderMinor int64  `json:"avgOrderMinor"`
}

func FromRestaurantDayRows(rows []analytics.RestaurantDayRow) []RestaurantDayResponseDTO {
    out := make([]RestaurantDayResponseDTO, 0, len(rows))
    for _, r := range rows {
        out = append(out, RestaurantDayResponseDTO{...})
    }
    return out
}
```

---

### 2.6 Repositories & Avoiding N+1 (`app/analytics/repository/`)

**Node/TS (`order-item.repo.ts` — Multi-Row SQL Insert):**
```ts
export async function bulkInsertItems(inputs: InsertOrderItemInput[], conn: Knex) {
    return conn("order_items").insert(inputs.map(...)).returning(ORDER_ITEM_COLUMNS);
}
```

**Go (`product_day.repo.go` — MongoDB `BulkWrite` Upsert):**
```go
models := make([]mongo.WriteModel, 0, len(orderProductIDs))
for _, pid := range orderProductIDs {
    models = append(models, mongo.NewUpdateOneModel().
        SetFilter(bson.M{"branch_id": branchID, "product_id": pid, "date": date}).
        SetUpdate(bson.M{"$inc": bson.M{"quantity_sum": qtyByProduct[pid], "revenue_sum": revByProduct[pid], "orders_count": int64(1)}}).
        SetUpsert(true))
}
_, err := r.coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
```

---

### 2.7 Dependency Injection (`lib/di/container.ts` vs `lib/boot/boot.go`)

**Node/TS (`container.ts`):**
```ts
container.registerSingleton(TOKENS.OrderService, OrderService);
```

**Go (`lib/boot/boot.go`):**
```go
restaurantRepo := repository.NewRestaurantDayRepo(mongoClient.DB())
branchRepo := repository.NewBranchDayRepo(mongoClient.DB())
productRepo := repository.NewProductDayRepo(mongoClient.DB())
platformRepo := repository.NewPlatformDayRepo(mongoClient.DB())
analyticsSvc := service.New(restaurantRepo, branchRepo, productRepo, platformRepo)
analyticsCtrl := controller.New(analyticsSvc)
```

---

## 3. Common Gotchas When Porting from TypeScript to Go

1. **`nil` Slice vs Empty Slice (`null` vs `[]` in JSON)**:
   - In Go, `var rows []DTO` has value `nil`, which `json.Marshal` serializes as `"data": null`.
   - Always allocate with `rows := make([]DTO, 0, len(src))` so empty results serialize as `"data": []`.
2. **Unordered `bson.M` vs Ordered `bson.D`**:
   - Go randomizes map iteration order on every run. Never pass `bson.M{"restaurant_id": 1, "date": 1}` to `Indexes().CreateMany` or `$sort` — use `bson.D{{Key: "restaurant_id", Value: 1}, {Key: "date", Value: 1}}`.
3. **Unexported Struct Fields Are Invisible to JSON/BSON/Validator**:
   - If you write `type DTO struct { ordersCount int64 }`, `encoding/json` will silently omit `ordersCount` because it starts with a lowercase letter. Exported fields must start with an uppercase letter (`OrdersCount int64`).
4. **Go Date Format Layout (`2006-01-02`)**:
   - Go does not use `YYYY-MM-DD`. You must pass `"2006-01-02"` (or `time.RFC3339`) to `time.Parse` and `time.Format`.
5. **In-Memory Shared State Requires a Mutex (`sync.RWMutex`)**:
   - Node.js runs your JS code on a single-threaded event loop, so `this.cache.set(role, perms)` in `PermissionCacheService` cannot data-race in memory.
   - Go handles every HTTP request and AMQP message on concurrent goroutines across multiple OS threads; concurrent map reads and writes without a `sync.RWMutex` will crash the process with `fatal error: concurrent map read and map write`.

---

## 4. When to Break the 1:1 Mapping

- **No DI Container**: Do not port `tsyringe` (`dig` / `fx`) to a small Go microservice. Explicit constructors in `lib/boot/boot.go` are clearer, faster, and compile-time checked.
- **No Separate `express.d.ts` Global Augmentation**: Use typed `context.Context` helper functions in `lib/appcontext` instead of string keys on `http.Request`.
- **Single-Binary Multi-Consumer vs Separate Worker Process**: Because Go goroutines are lightweight and multiplexed onto OS threads, `analytics-service` runs its AMQP consumers and HTTP server cleanly in the same process (`cmd/api`), while heavy batch jobs (`cmd/backfill-aggs`) live as a separate `cmd/` binary sharing the same `pkg/`, `lib/`, and `app/` packages.

