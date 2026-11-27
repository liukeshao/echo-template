# AGENTS.md

Rules and invariants for AI agents working in this repository.

---

## Architecture

- **Layers**: `api/*.tsp (TypeSpec)` → `pkg/api/*.gen.go (generated pure DTOs + net/http glue)` → `internal/server (thin HTTP wiring: BuildRouter, NewProblemEncoder, Locale middleware, BearerAuth, Docs, Server lifecycle)` → `internal/handler (implements api.XxxHandler, pure Go zero net/http, multi-service orchestration & DTO projection)` → `internal/service (domain logic)` → `ent ORM` / `internal/client/<api>`. Never skip layers (no DB calls or HTTP calls in handlers).
- **Public vs Private Boundaries**: `pkg/api`, `pkg/httpclient`, `pkg/httperr`, `pkg/httpkit`, `pkg/logging`, and `pkg/validator` are pure, reusable public packages with **zero** `internal/` or third-party dependencies. All application-specific wiring, config, DI, ORM, i18n, and business logic live under `internal/`.
- **HTTP Transport & Error Split**: Pure Go 1.22+ `net/http` (`*http.ServeMux`) is split cleanly into three `http*` packages: `pkg/httpclient` (outbound HTTP client & retry transport), `pkg/httperr` (13 finite HTTP error status sentinels, pure RFC 9457 `httperr.Problem`, AIP-193 `httperr.Metadata`, `httperr.NewEncoder` three-stage pipeline / `Classify` / `Write`), and `pkg/httpkit` (inbound `httpkit.Server` graceful shutdown, `httpkit.Router`, `httpkit.Handler`, `@useAuth` middleware, base middlewares including `RequestID`/`AccessLog`/`Recover`/`BodyLimit`, stateless codecs, RFC 7386 Merge Patch). `internal/server` acts solely as a thin 4-file application wiring layer (`package.go` wiring `httpkit.Server`, `router.go` with `BuildRouter`/`BearerAuth`/`Locale`, `problem.go` wiring `httperr.NewEncoder`, `docs.go`). Domain services, outbound clients, and `i18n` depend ONLY on `pkg/httperr`, never on `pkg/httpkit`. No third-party HTTP framework.
- **Connect-Go v2 Style**: Handlers (`handler.Auth`, `handler.Me`, `handler.Book`) implement pure Go interfaces (`api.AuthHandler`, `api.MeHandler`, `api.BooksHandler`) with zero `net/http` or `Mount` boilerplate. Generated `api.NewXxxHandler(impl, opts...) httpkit.Handler` handles routing, auth wrapping, zero-reflection decoding, `Validate()`, `@autoPatch`, JSON encoding, and RFC 9457 errors via `httperr.Encoder`.
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

- Each third-party API lives in `internal/client/<apiname>/` with four files: `interface.go`, `client.go`, `errors.go`, `package.go`.
- Services depend on the **interface** (`<apiname>.API`), never on `*Client` directly.
- Config goes in `config.Clients.<ApiName>` (type `config.ExternalClient`); add a `[clients.<apiname>]` block in `config.toml`.
- Map upstream failures to the finite HTTP error statuses in `pkg/httperr`: HTTP 429 → `httperr.TooManyRequests` (with `httperr.RetryInfo{Service, RetryAfter}`), 502/503/504 and network errors → `httperr.ServiceUnavailable`, everything else → `httperr.Internal`. Optionally add fine-grained i18n copy under `[details.429]` in `internal/i18n/messages/*.toml`.
- Derive a per-API HTTP client via `shared.WithOptions(httpclient.Options{Timeout: cfg.Timeout, Retry: cfg.Retry})`. The transport automatically retries idempotent requests on transient network errors and HTTP 429/502/503/504 with exponential backoff and jitter (within the total `Timeout`).
- Register the package in `internal/client/package.go` and wire `client.Package` in `cmd/server/main.go` between `database.Package` and `service.Package`.
- **Forbidden**: raw `http.Get` / `http.Post` calls anywhere outside `internal/client/`.

---

## Error Handling (Pure RFC 9457 + Finite HTTP Statuses + AIP-193 Metadata)

All errors use the **13 finite HTTP error statuses** defined in `pkg/httperr/status.go` (`httperr.Status`, backed directly by `int` HTTP status codes: `BadRequest` 400, `Unauthorized` 401, `Forbidden` 403, `NotFound` 404, `MethodNotAllowed` 405, `Conflict` 409, `ContentTooLarge` 413, `UnsupportedMediaType` 415, `TooManyRequests` 429, `ClientClosedRequest` 499, `Internal` 500, `ServiceUnavailable` 503, `GatewayTimeout` 504). Each `httperr.Status` implements `error` (immutable zero-allocation sentinel for `errors.Is`) — no duplicate `Code` string, no `Code ↔ HTTPStatus` switch table, and no global registry or `init()`.

Errors are serialized by `httperr.NewEncoder` (wired in `server.NewProblemEncoder` with `logging.RequestID`, `oops` metadata extractor, and `internal/i18n` translator) as pure RFC 9457 `httperr.Problem` (`Content-Type: application/problem+json`: `type`, `title`, `status`, `detail`, `instance`, `request_id`, `metadata`, `errors`) through a three-stage pipeline (`Classify` → `Sanitize & Log` → `Unified Localize`). Classification precedence:

1. Request context cancellation/timeout (`r.Context().Err() != nil && errors.Is(err, r.Context().Err())`) → `499` / `504` (prevents mid-query client disconnects from triggering false 500 alerts)
2. `httperr.Classify(err)` (`httperr.Problem`, `*validator.FieldViolations` → `400` + `httperr.ValidationInfo` + `errors[]`, `*http.MaxBytesError` → `413` + `httperr.RequestLimitInfo`, explicit `httperr.Status` sentinel from `.Wrap`/`.Join` ahead of bare `context.DeadlineExceeded`/`Canceled`, and default `500`)
3. For 4xx `oops.OopsError` (`oops.AsOops`) with empty `Problem.Metadata`, extract typed `httperr.Metadata` strictly from `oopsErr.Context()[httperr.MetaKey]` (all other `oops.With` / `oops.User` / `oops.In` / `oops.Tags` stay internal to `slog` logs)

Unmatched `http.ServeMux` routes (404 with `ResourceInfo` / 405 with `MethodInfo`, while preserving 301/307/308 redirects) and recovered panics (500) are also converted to RFC 9457 `application/problem+json`. `httperr.Write` automatically sets `Retry-After` from `httperr.RetryInfo`, and `ProblemEncoder` sets `Content-Language` from the matched locale. All user-facing copy (`Problem.Detail` and `FieldError.Message`) is rendered **solely** at the final `localizeProblem` stage via `internal/i18n` (`[details.<STATUS>]` hierarchical subkeys → placeholder-free `[errors]` status fallback + `[validation]`); `oops.Public(...)` is never read.

**Service layer error pattern:**

```go
var errDomain = oops.In("domain")

// internal error — internal debug context logged via slog, never exposed in Problem.Metadata
errDomain.Tags("database").With("domain_id", id, "op", op).Join(httperr.Internal, err)

// client error — wrap httperr.Status sentinel + attach AIP-193 httperr.Metadata under httperr.MetaKey; NEVER call .Public(...)
errDomain.
	With(httperr.MetaKey, httperr.ResourceInfo{
		ResourceType: "domain",
		ResourceName: id,
	}).
	Wrap(httperr.NotFound)
```

**Rules:**
- Never use bare `fmt.Errorf` or `errors.New` in service layer.
- Never call `oops.Public(...)` or hardcode user-facing English/Chinese sentences in services, clients, middleware, `pkg/httperr`, or `pkg/httpkit`.
- Never invent per-domain error code registries or duplicate `Code` fields alongside HTTP `status` (use the 13 finite `httperr.Status` sentinels + AIP-193 `httperr.Metadata` structs).
- Always wrap the `httperr.Status` sentinel via `.Wrap(httperr.NotFound)` or `.Join(httperr.Internal, cause)` (no redundant `.Code(...)`) so both `errors.Is(err, httperr.NotFound)` and `httperr.Classify(err)` share one single source of truth.
- Never return `*ent.NotFoundError` directly to handlers.
- Store internal actor IDs with `oops.User(userID)` and internal diagnostic fields with arbitrary `oops.With("k", v)` keys; only values implementing `httperr.Metadata` (with a **value receiver**, as `oops.Context()` dereferences pointers) under `httperr.MetaKey` are exposed to HTTP clients (and stripped on 5xx).
- Startup `assertCoverage` enforces 100% coverage for all 13 placeholder-free `[errors]` HTTP statuses and all `[validation]` rules in `internal/i18n/messages/*.toml`; fine-grained domain messages go under `[details.<STATUS>]` with cross-locale key and placeholder parity checked in tests.

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
- Mapping functions (`toXxx`) belong in `internal/handler/<domain>.go`, not in services. Use `mapSlice(items, toXxx)` instead of per-domain slice mappers.

---

## Validation

- Single-field constraints (`UUID`, `Email`, `Username`, `Password`, `@minLength`, `@maxLength`, `@pattern`, `@minValue`, `@maxValue`) are declared in TypeSpec (`api/spec/`) and compiled into `Validate() error` calling declarative `pkg/validator` checks (`Required`, `MinLength`, `MaxLength`, `MaxBytes`, `Pattern`, `UUID`, `Email`, `PasswordComplexity`, `Min`, `Max`, `OneOf`) and returning `*validator.FieldViolations` with strongly-typed `validator.Rule` codes.
- Cross-field rules live in hand-written `pkg/api/<domain>_validate.go` via `validateExtra(*validator.FieldViolations)` (e.g. `v.MustDiffer(...)`).
- List sorting (`order_by`) uses `validator.ParseOrderBy("order_by", req.OrderBy, allowedFields...)` to validate and parse `"field"` / `"field desc"` expressions into `[]validator.OrderTerm`.
- Generated `api.NewXxxHandler` runs `Validate()` automatically before invoking the handler method.

---

## Auth

- `server.BearerAuth` (`httpkit.Authenticator`, defined alongside router wiring in `internal/server/router.go`) validates Bearer JWT tokens and enriches context via `handler.WithUser`.
- Route-level auth is driven declaratively by `@useAuth(...)` in TypeSpec; generated `api.NewXxxHandler` emits `httpkit.Operation{ID, Method, Pattern, Auth}` and delegates to `cfg.Wrap(op, ...)`.
- Register named `@useAuth` schemes in `internal/server/router.go` via `httpkit.WithAuthScheme("BearerAuth", ...)` (natively supporting single, `AND`, `OR`, and `NoAuth` combinations with fail-fast checks on unregistered schemes), or plug in any third-party `net/http` middleware per-operation via `httpkit.WithMiddleware(func(op httpkit.Operation) httpkit.Middleware)`.
- In handlers behind `@useAuth(BearerAuth)`, use `handler.MustUser` — never the non-panicking variant.
- JWT claims (`jwtClaims` in `internal/service/auth.go`) contain only `user_id` and `token_type`. Do not add user fields to claims.
- Token types use `ent/token.Type` enum. Do not use raw strings like `"access"`.

---

## Logging

- `log/slog` with `ContextHandler` (`pkg/logging`) — always pass `ctx` to `slog.InfoContext` / `slog.ErrorContext`.
- `request_id` is injected into context by `httpkit.RequestID()` (via `logging.WithRequestID`) and surfaces automatically in both logs and `Problem.RequestID`.
- Never call `slog.Info` (without context) in request-scoped code.

---

## Testing

- Use `internal/testutil` in external test packages (`package service_test`, `package handler_test`, `package <api>_test`) instead of re-creating SQLite/Ent/DI setup manually:
  - `env := testutil.New(t, ...)` builds the full lazy DI container with an isolated in-memory SQLite DB and automatic `t.Cleanup` shutdown (safe for `t.Parallel()`).
  - Use `testutil.WithConfig(...)`, `testutil.WithOverride[T](mock)`, or `testutil.WithHTTPTransport(rt)` to customize config or inject mocks.
  - Handler tests use `env.MustAuthUser(...)`, `env.DoRequest(...)`, `testutil.DecodeJSON[T](t, rec)`, and `testutil.AssertProblem(...)`.
  - Service/client tests use `testutil.AssertOopsStatus(t, err, httperr.NotFound)` (which asserts `oops.AsOops`, `errors.Is`, and the primary wrapped `httperr.Status` sentinel).

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

Unsupported media types return `415 Unsupported Media Type`.

Annotate the `patch` operation in TypeSpec with `@autoPatch(Domain.getXxx, Domain.replaceXxx)`:
- OpenAPI 3.1 documents `PATCH` with the typed `PatchXxxRequest` schema and `415`.
- The generated `api.<Domain>Handler` interface **omits** `PatchXxx` — handlers only implement `GetXxx` and `ReplaceXxx`.
- Generated `api.New<Domain>Handler` automatically executes `GetXxx` → `httpkit.ApplyPatch` → restore path param → `Validate()` → `ReplaceXxx`.
- Read-only fields (`id`, `created_at`, `updated_at`, etc.) are absent from `ReplaceXxxRequest`, so patches attempting to mutate them are silently dropped during unmarshalling.

### List methods

- **Pagination**: `page int32` (1-based, default 1, reject if negative) + `page_size int32` (default 50, max 1000; coerce if over max, reject if negative). Service `List(ctx, req)` methods normalize pagination via `resolvePagination` in `internal/service/pagination.go` and return `*service.Page[*ent.Xxx]`.
- **Ordering**: `order_by string` — comma-separated `"field"` or `"field desc"`. Return `*validator.FieldViolations` on errors.
- **Query fields**: Declare resource-specific `@query` fields directly on each `listXxx(...ListQuery, @query ...)` operation in TypeSpec (e.g. `title`, `author`, `min_price`, `max_price`), and translate them directly into typed ent predicates inside the owning service's `List` method; never build raw SQL.

---

## Forbidden Patterns

| Pattern | Reason |
|---|---|
| `fmt.Errorf` in service layer | Breaks error classification pipeline |
| `oops.Public(...)` or hardcoded `Problem.Detail` | Bypasses unified terminal i18n; pass `httperr.Status` + `httperr.Metadata` instead |
| Returning `*ent.Xxx` from handler | Leaks internal fields |
| `bcrypt` calls outside `internal/service/password.go` | Centralised cost policy (cost=12) |
| Per-domain custom error code registries | Use the 13 finite `httperr.Status` sentinels + AIP-193 `httperr.Metadata` |
| Storing `context.Context` in struct | Violates Go context contract |
| Adding user data to JWT claims | Over-exposure; trust DB as source of truth |
| Manually editing `pkg/api/*.gen.go` or `static/openapi/openapi.yaml` | Generated by `make api`; manual edits are overwritten |
| Touching `http.Request`/`http.ResponseWriter` or writing `Mount` inside domain handlers | Generated `api.NewXxxHandler` + `pkg/httpkit` + `internal/server` own HTTP transport |
| Hand-writing `PatchXxx` in handlers | Use `@autoPatch(GetOp, ReplaceOp)` in TypeSpec so RFC 7386 Merge Patch converges on `Get` + `Replace` |
| camelCase URL verb segments (e.g. `/resetPassword`) | Use kebab-case (`/reset-password`) |
| Raw `http.Get`/`http.Post` outside `internal/client/` | Bypasses shared transport, retry, and error classification |
| Depending on `*Client` concrete type in services | Use the `API` interface; keeps services testable |
