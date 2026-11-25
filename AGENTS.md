# AGENTS.md

Rules and invariants for AI agents working in this repository.

---

## Architecture

- **Layers**: `api/*.tsp (TypeSpec)` → `pkg/api/*.gen.go (generated net/http glue)` → `pkg/handlers (implements api.XxxHandler, multi-service orchestration & DTO projection)` → `pkg/services (domain logic)` → `ent ORM` / `pkg/clients/<api>`. Never skip layers (no DB calls or HTTP calls in handlers).
- **HTTP Transport**: Pure Go 1.22+ `net/http` (`*http.ServeMux`) encapsulated by `pkg/rest` (`rest.Router`, `rest.Runtime`, `rest.Handler`, `rest.Server`). No third-party HTTP framework.
- **Connect-Go v2 Style**: Handlers (`handlers.Auth`, `handlers.Me`, `handlers.Book`) implement pure Go interfaces (`api.AuthHandler`, `api.MeHandler`, `api.BooksHandler`) with zero `net/http` or `Mount` boilerplate. Generated `api.NewXxxHandler(impl, opts...) rest.Handler` handles routing, auth wrapping, zero-reflection decoding, `Validate()`, `@autoPatch`, JSON encoding, and RFC 9457 errors.
- **DI**: All singletons are registered via `samber/do`. Use `do.Lazy(NewFoo)` for lazy init; use `do.Provide(NewFoo)` only when eager init is necessary.
- **Context**: Always propagate `context.Context` as the first argument. Never store it in struct fields.

---

## TypeSpec & Code Generation

- **Single Source of Truth**: `api/spec/` (`main.tsp`, `common/*.tsp`, `domains/*.tsp`) defines all routes, models, semantic scalars (`UUID`, `Email`, `Username`, `Password`), validation constraints, auth (`@useAuth(BearerAuth)`), and auto-patch (`@autoPatch(GetOp, ReplaceOp)`).
- **Custom Emitter**: `api/emitters/go-rest/` (ESM, no build step) runs alongside `@typespec/openapi3` via a single command:

```bash
# After changing api/spec/**/*.tsp or api/emitters/go-rest/**
make api
```

```bash
# After changing ent/schema/*.go
make ent
```

- **Do not manually edit** `pkg/api/*.gen.go`, `static/openapi/openapi.yaml`, or files under `ent/` (except `ent/schema/`).
- For cross-field validation that cannot be expressed as single-field TypeSpec decorators (e.g. `old_password != new_password`), add an unexported `validateExtra(v *validator.FieldViolations)` method in a hand-written `pkg/api/<domain>_validate.go` file — generated `Validate()` invokes it automatically.

---

## External API Clients

- Each third-party API lives in `pkg/clients/<apiname>/` with four files: `interface.go`, `client.go`, `errors.go`, `package.go`.
- Services depend on the **interface** (`<apiname>.API`), never on `*Client` directly.
- Config goes in `config.ClientsConfig.<ApiName>` (type `ExternalClientConfig`); add a `[clients.<apiname>]` block in `config.toml`.
- Error codes go in `pkg/codes/<apiname>.go`. Typical set: `<API>_UNAVAILABLE` → `http.StatusServiceUnavailable` (503), `<API>_RATE_LIMITED` → `http.StatusTooManyRequests` (429), `<API>_INTERNAL_ERROR` → `http.StatusInternalServerError` (500).
- Use `infra.Retry(ctx, cfg.Retry+1, fn)` for transient retries (exponential backoff with jitter). Wrap non-retryable errors (4xx, JSON decode errors) in `infra.Permanent(err)`. Map HTTP 429 → `<api>RateLimited`, 502/503/504 and network errors → `<api>Unavailable`, everything else → `infra.Permanent(<api>Internal)`.
- Register the package in `pkg/clients/package.go` and add it to `cmd/main.go` between `infra.Package` and `services.Package`.
- **Forbidden**: raw `http.Get` / `http.Post` calls anywhere outside `pkg/clients/`.

---

## Error Handling (RFC 9457 Problem Details)

All errors are serialized by `pkg/rest.Runtime.WriteProblem` as flat, strongly-typed `rest.Problem` (`Content-Type: application/problem+json`). Classification precedence:

1. `rest.Problem` / `*rest.Problem` — used as-is (defaults filled for `type`, `title`, `instance`, `request_id`)
2. `oops.OopsError` with `.Code()` — maps via `codes.HTTPStatus()`, localizes `detail` via `pkg/i18n` using `codes.CtxKeyParams`, exposes `params` on 4xx only
3. `*validator.FieldViolations` — `400 VALIDATION_INVALID` with localized `errors: []rest.FieldError`
4. `*http.MaxBytesError` — `413 REQUEST_ENTITY_TOO_LARGE`
5. `context.DeadlineExceeded` / `context.Canceled` — `504 GATEWAY_TIMEOUT` / `499 CLIENT_CLOSED_REQUEST`
6. Everything else — `500 INTERNAL_ERROR` (internal details hidden, localized fallback message)

Unmatched `http.ServeMux` routes (404 / 405) and recovered panics (500) are also converted to RFC 9457 `application/problem+json` by `rest.Router`.

**Service layer error pattern:**

```go
// internal error — hide details
oops.Code(codes.DomainInternal.String()).Tags("domain").Wrapf(err, "%s", op)

// client error — attach structured params for i18n interpolation & Problem.Params
oops.Code(codes.DomainNotFound.String()).
	With(codes.CtxKeyParams, map[string]string{"domain_id": id}).
	Public(fmt.Sprintf("Domain %q not found.", id)).
	Errorf("domain not found")
```

**Rules:**
- Never use bare `fmt.Errorf` or `errors.New` in service layer.
- Never return `*ent.NotFoundError` directly to handlers.
- Every new error code must be registered in `pkg/codes/<domain>.go` `init()` with a 4xx/5xx `net/http` status constant, and have a corresponding entry in `pkg/i18n/messages/en.toml` (and `zh-CN.toml`).

---

## Entity & DTO

- Ent schema files live in `ent/schema/`. Always use `DefaultMixin` (UUID v7 primary key + soft delete + timestamps).
- **Never return `*ent.Xxx` from handlers.** Always project to generated flat domain-noun DTOs in `pkg/api` (`api.Book`, `api.BookList`, `api.User`, `api.TokenPair`).
- Keep DTO structs **flat** (no embedded `ObjectMeta` / `XxxSpec` wrappers).
- Naming convention in `pkg/api`:
  - **Requests**: `*Request` (`CreateBookRequest`, `ReplaceBookRequest`, `ListBooksRequest`, `GetBookRequest`, `LoginRequest`).
  - **Responses**: pure domain nouns (`Book`, `BookList`, `User`, `TokenPair`, `Problem`) — no `*Response` suffix.
  - Keep internal structs (such as `jwtClaims`) inside their owning service package, never in `pkg/api`.
- Resource DTOs must not expose `password_hash`, `deleted_at`, or other internal fields.
- Mapping functions (`toXxx`) belong in `pkg/handlers/<domain>.go`, not in services. Use `mapSlice(items, toXxx)` instead of per-domain slice mappers.

---

## Validation

- Single-field constraints (`UUID`, `Email`, `Username`, `Password`, `@minLength`, `@maxLength`, `@minValue`) are declared in TypeSpec (`api/spec/`) and compiled into `Validate() error` returning `*validator.FieldViolations`.
- Cross-field rules live in hand-written `pkg/api/<domain>_validate.go` via `validateExtra(*validator.FieldViolations)`.
- Generated `api.NewXxxHandler` runs `Validate()` automatically before invoking the handler method.

---

## Auth

- `handlers.BearerAuth` (`rest.Authenticator`, defined alongside router wiring in `pkg/handlers/router.go`) validates Bearer JWT tokens and enriches context via `appctx.WithUser`.
- Route-level auth is driven declaratively by `@useAuth(...)` in TypeSpec; generated `api.NewXxxHandler` emits `rest.Operation{ID, Method, Pattern, Auth}` and delegates to `cfg.Wrap(op, ...)`.
- Register named `@useAuth` schemes in `pkg/handlers/router.go` via `rest.WithAuthScheme("BearerAuth", ...)` (natively supporting single, `AND`, `OR`, and `NoAuth` combinations with fail-fast checks on unregistered schemes), or plug in any third-party `net/http` middleware per-operation via `rest.WithMiddleware(func(op rest.Operation) rest.Middleware)`.
- In handlers behind `@useAuth(BearerAuth)`, use `appctx.MustGetUserFromContext` — never the non-panicking variant.
- JWT claims (`jwtClaims` in `pkg/services/auth.go`) contain only `user_id` and `token_type`. Do not add user fields to claims.
- Token types use `ent/token.Type` enum. Do not use raw strings like `"access"`.

---

## Logging

- `log/slog` with `ContextHandler` — always pass `ctx` to `slog.InfoContext` / `slog.ErrorContext`.
- `request_id` is injected into context by `rest.RequestID()` and surfaces automatically in both logs and `Problem.RequestID`.
- Never call `slog.Info` (without context) in request-scoped code.

---

## Testing

- Use `pkg/testutil` in external test packages (`package services_test`, `package handlers_test`, `package <api>_test`) instead of re-creating SQLite/Ent/DI setup manually:
  - `env := testutil.New(t, ...)` builds the full lazy DI container with an isolated in-memory SQLite DB and automatic `t.Cleanup` shutdown (safe for `t.Parallel()`).
  - Use `testutil.WithConfig(...)`, `testutil.WithOverride[T](mock)`, or `testutil.WithHTTPTransport(rt)` to customize config or inject mocks.
  - Handler tests use `env.MustAuthUser(...)`, `env.DoRequest(...)`, `testutil.DecodeJSON[T](t, rec)`, and `testutil.AssertProblem(...)`.
  - Service/client tests use `testutil.AssertOopsCode(t, err, codes.Xxx)`.

---

## API Style — Pragmatic REST

Each resource type owns a collection URL, the URL path identifies the resource, and the HTTP method expresses the action. Custom verbs are kebab-case sub-paths beneath the resource. Partial updates use `PATCH` with RFC 7386 JSON Merge Patch.

### Method semantics

| Method  | Use for                                  | Idempotent | Body                                                              |
|---------|------------------------------------------|------------|-------------------------------------------------------------------|
| POST    | Create / non-idempotent custom verbs     | No         | `CreateXxxRequest` (or custom-verb DTO)                           |
| GET     | Read single / collection                 | Yes        | none                                                              |
| PUT     | Full replacement of an existing resource | Yes        | `ReplaceXxxRequest` (all writable fields)                         |
| PATCH   | Partial update                           | Yes        | RFC 7386 JSON Merge Patch (`PatchXxxRequest`)                     |
| DELETE  | Remove                                   | Yes        | none                                                              |

Singleton resources (e.g. `/api/v1/me` bound to the current user) skip POST and DELETE — only GET / PUT / PATCH plus custom verbs.

### PATCH (`@autoPatch`, RFC 7386 JSON Merge Patch)

Supported `Content-Type` values:

| Content-Type                   | Body format                                                    |
|--------------------------------|----------------------------------------------------------------|
| `application/merge-patch+json` | RFC 7386 JSON Merge Patch — omit to keep, `null` to clear      |
| `application/json` / empty     | Compatibility: treated as RFC 7386 JSON Merge Patch            |

Unsupported media types return `415 Unsupported Media Type` (`PATCH_UNSUPPORTED_MEDIA_TYPE`).

Annotate the `patch` operation in TypeSpec with `@autoPatch(Domain.getXxx, Domain.replaceXxx)`:
- OpenAPI 3.1 documents `PATCH` with the typed `PatchXxxRequest` schema and `415`.
- The generated `api.<Domain>Handler` interface **omits** `PatchXxx` — handlers only implement `GetXxx` and `ReplaceXxx`.
- Generated `api.New<Domain>Handler` automatically executes `GetXxx` → `rt.ApplyPatch` → restore path param → `Validate()` → `ReplaceXxx`.
- Read-only fields (`id`, `created_at`, `updated_at`, etc.) are absent from `ReplaceXxxRequest`, so patches attempting to mutate them are silently dropped during unmarshalling.

### List methods

- **Pagination**: `page int32` (1-based, default 1, reject if negative) + `page_size int32` (default 50, max 1000; coerce if over max, reject if negative). Service `List(ctx, req)` methods normalize pagination via `resolvePagination` in `pkg/services/pagination.go` and return `*services.Page[*ent.Xxx]`.
- **Ordering**: `order_by string` — comma-separated `"field"` or `"field desc"`. Return `*validator.FieldViolations` on errors.
- **Query fields**: Declare resource-specific `@query` fields directly on each `listXxx(...ListQuery, @query ...)` operation in TypeSpec (e.g. `title`, `author`, `min_price`, `max_price`), and translate them directly into typed ent predicates inside the owning service's `List` method; never build raw SQL.

---

## Forbidden Patterns

| Pattern | Reason |
|---|---|
| `fmt.Errorf` in service layer | Breaks error classification pipeline |
| Returning `*ent.Xxx` from handler | Leaks internal fields |
| `bcrypt` calls outside `pkg/services/password.go` | Centralised cost policy (cost=12) |
| Registering duplicate error codes | `init()` panics by design |
| Storing `context.Context` in struct | Violates Go context contract |
| Adding user data to JWT claims | Over-exposure; trust DB as source of truth |
| Manually editing `pkg/api/*.gen.go` or `static/openapi/openapi.yaml` | Generated by `make api`; manual edits are overwritten |
| Touching `http.Request`/`http.ResponseWriter` or writing `Mount` inside domain handlers | Generated `api.NewXxxHandler` + `pkg/rest` own HTTP transport |
| Hand-writing `PatchXxx` in handlers | Use `@autoPatch(GetOp, ReplaceOp)` in TypeSpec so RFC 7386 Merge Patch converges on `Get` + `Replace` |
| camelCase URL verb segments (e.g. `/resetPassword`) | Use kebab-case (`/reset-password`) |
| Raw `http.Get`/`http.Post` outside `pkg/clients/` | Bypasses shared transport, retry, and error classification |
| Depending on `*Client` concrete type in services | Use the `API` interface; keeps services testable |
