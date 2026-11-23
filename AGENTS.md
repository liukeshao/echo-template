# AGENTS.md

Rules and invariants for AI agents working in this repository.

---

## Architecture

- **Layers**: `handler` → `service` → `ent ORM` / `pkg/clients/<api>`. Never skip layers (no DB calls or HTTP calls in handlers).
- **DI**: All singletons are registered via `samber/do`. Use `do.Lazy(NewFoo)` for lazy init; use `do.Provide(NewFoo)` only when eager init is necessary.
- **Context**: Always propagate `context.Context` as the first argument. Never store it in struct fields.

---

## External API Clients

- Each third-party API lives in `pkg/clients/<apiname>/` with four files: `interface.go`, `client.go`, `errors.go`, `package.go`.
- Services depend on the **interface** (`<apiname>.API`), never on `*Client` directly.
- Config goes in `config.ClientsConfig.<ApiName>` (type `ExternalClientConfig`); add a `[clients.<apiname>]` block in `config.toml`.
- Error codes go in `pkg/codes/<apiname>.go`. Typical set: `<API>_UNAVAILABLE` → `http.StatusServiceUnavailable` (503), `<API>_RATE_LIMITED` → `http.StatusTooManyRequests` (429), `<API>_INTERNAL_ERROR` → `http.StatusInternalServerError` (500).
- Use `utils.Retry(ctx, cfg.Retry+1, fn)` for transient retries (exponential backoff with jitter). Wrap non-retryable errors (4xx, JSON decode errors) in `utils.Permanent(err)`. Map HTTP 429 → `<api>RateLimited`, 502/503/504 and network errors → `<api>Unavailable`, everything else → `utils.Permanent(<api>Internal)`.
- Register the package in `pkg/clients/package.go` and add it to `cmd/main.go` between `infra.Package` and `services.Package`.
- **Forbidden**: raw `http.Get` / `http.Post` calls anywhere outside `pkg/clients/`.

---

## Error Handling (RFC 9457 Problem Details)

All errors are serialized by `error_handler` as flat, strongly-typed `types.Problem` (`Content-Type: application/problem+json`). Classification precedence:

1. `types.Problem` / `*types.Problem` — used as-is (defaults filled for `type`, `title`, `instance`, `request_id`)
2. `oops.OopsError` with `.Code()` — maps via `codes.HTTPStatus()`, localizes `detail` via `pkg/i18n` using `codes.CtxKeyParams`, exposes `params` on 4xx only
3. `*validator.FieldViolations` — `400 VALIDATION_INVALID` with localized `errors: []types.FieldError`
4. `context.DeadlineExceeded` / `context.Canceled` — `504 GATEWAY_TIMEOUT` / `499 CLIENT_CLOSED_REQUEST`
5. `echo.HTTPStatusCoder` — preserves exact HTTP status (e.g. `404`, `405`, `413`)
6. Everything else — `500 INTERNAL_ERROR` (internal details hidden, localized fallback message)

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

- Ent schema files live in `ent/schema/`. Always use `DefaultMixin` (ULID primary key + soft delete + timestamps).
- **Never return `*ent.Xxx` from handlers.** Always map to a flat domain-noun DTO in `pkg/types` (`types.Book`, `types.BookList`, `types.User`, `types.TokenPair`).
- Keep DTO structs **flat** (do not embed `ObjectMeta` / `XxxSpec` wrappers) so Go struct literals stay ergonomic and Create / Replace / Response schemas can evolve independently.
- Naming convention in `pkg/types`:
  - **Requests**: `*Request` (`CreateBookRequest`, `ReplaceBookRequest`, `ListRequest`, `LoginRequest`), or `*Path` (`BookPath`) for path-only param binders.
  - **Responses**: pure domain nouns (`Book`, `BookList`, `User`, `TokenPair`, `Problem`) — no `*Response` suffix.
  - Keep internal structs (such as `jwtClaims`) inside their owning service package, never in `pkg/types`.
- Resource DTOs must not expose `password_hash`, `deleted_at`, or other internal fields.
- Resource DTO doubles as the JSON view used by PATCH (read-modify-write); read-only fields (`id`, `created_at`, `updated_at`, etc.) are absent from `ReplaceXxxRequest`, so patches that mutate them are silently dropped during unmarshalling.
- Mapping functions (`toXxx`) belong in `pkg/handlers/<domain>.go`, not in services. Use `mapSlice(items, toXxx)` instead of per-domain slice mappers.

---

## Validation

Request structs implement `Validate() error` returning `*validator.FieldViolations` (localized by `error_handler` via `pkg/i18n` into `Problem.Errors`).

```go
func (r *CreateXxxRequest) Validate() error {
	var v validator.FieldViolations
	if strings.TrimSpace(r.Title) == "" {
		v.Add("title", validator.KeyRequired, map[string]string{"field": "title"})
	} else if len(r.Title) > 200 {
		v.Add("title", validator.KeyMaxLength, map[string]string{"field": "title", "max": "200"})
	}
	return v.Err()
}
```

- Use `bindAndValidate[types.XxxRequest](c)` in handlers to bind and validate in one step.
- Use `validator.FieldViolations` with i18n key constants (`validator.Key*`) — not `echo.Bind` validator, not direct string errors.

---

## Auth

- `RequireAuth` middleware sets the authenticated `*ent.User` in context via `appctx.WithUser`.
- In handlers behind `RequireAuth`, use `appctx.MustGetUserFromContext` — never the non-panicking variant.
- JWT claims (`jwtClaims` in `pkg/services/auth.go`) contain only `user_id` and `token_type`. Do not add user fields to claims.
- Token types use `ent/token.Type` enum. Do not use raw strings like `"access"`.

---

## Logging

- `log/slog` with `ContextHandler` — always pass `ctx` to `slog.InfoContext` / `slog.ErrorContext`.
- `request_id` is injected into context by `middleware.RequestContext()` and surfaces automatically in both logs and `types.Problem.RequestID`.
- Never call `slog.Info` (without context) in request-scoped code.

---

## Testing

- Use `pkg/testutil` in external test packages (`package services_test`, `package handlers_test`, `package <api>_test`) instead of re-creating SQLite/Ent/DI setup manually:
  - `env := testutil.New(t, ...)` builds the full lazy DI container with an isolated in-memory SQLite DB and automatic `t.Cleanup` shutdown (safe for `t.Parallel()`).
  - Use `testutil.WithConfig(...)`, `testutil.WithOverride[T](mock)`, or `testutil.WithHTTPTransport(rt)` to customize config or inject mocks.
  - Handler tests use `env.MustAuthUser(...)`, `env.DoRequest(...)`, `testutil.DecodeJSON[T](t, rec)`, and `testutil.AssertProblem(...)`.
  - Service/client tests use `testutil.AssertOopsCode(t, err, codes.Xxx)`.

---

## Code Generation

```bash
# After changing ent/schema/*.go
make ent
```

```bash
# After changing Swaggo annotations (handler godoc comments)
make swag
```

- Do not manually edit files under `ent/` (except `ent/schema/`).
- Do not manually edit `static/openapi/openapi.yaml`. It is generated by `make swag` from Swaggo annotations in `pkg/handlers/*.go`. Manual edits will be overwritten on the next run.

---

## API Style — Pragmatic REST

Each resource type owns a collection URL, the URL path identifies the resource, and the HTTP method expresses the action. Custom verbs are sub-paths beneath the resource. Partial updates use PATCH with `Content-Type` selecting one of three patch protocols.

### Method semantics

| Method  | Use for                                  | Idempotent | Body                                                              |
|---------|------------------------------------------|------------|-------------------------------------------------------------------|
| POST    | Create / non-idempotent custom verbs     | No         | `CreateXxxRequest` (or custom-verb DTO)                           |
| GET     | Read single / collection                 | Yes        | none                                                              |
| PUT     | Full replacement of an existing resource | Yes        | `ReplaceXxxRequest` (all writable fields)                         |
| PATCH   | Partial update                           | Depends    | RFC 6902 / RFC 7386 / Shorthand — selected by `Content-Type`      |
| DELETE  | Remove                                   | Yes        | none                                                              |

Singleton resources (e.g. `/api/v1/me` bound to the current user) skip POST and DELETE — only GET / PUT / PATCH plus custom verbs.

### Routes

```go
g.POST("",                  h.Create)
g.GET("",                   h.List)
g.GET("/:book",             h.Get)
g.PUT("/:book",             h.Replace)        // full replacement
g.PATCH("/:book",           h.Patch)          // RFC 6902 / 7386 / Shorthand
g.DELETE("/:book",          h.Delete)

// Custom methods: verb as a kebab-case path segment.
g.POST("/:book/archive",    h.Archive)        // resource-level action
g.POST("/batch-get",        h.BatchGet)       // collection-level action
```

- Path parameters use the resource type in singular form (`/:book`, not `/:id`).
- Verb segments are **kebab-case** (`reset-password`, `batch-get`), never camelCase.
- Method choice: idempotent transitions → `PUT`, non-idempotent actions → `POST`, sub-state reads → `GET`.

### PATCH protocols

The `PATCH` handler dispatches on `Content-Type`. Supported values:

| Content-Type                        | Body format                                               |
|-------------------------------------|-----------------------------------------------------------|
| `application/json-patch+json`       | RFC 6902 — array of `{op,path,value}` operations          |
| `application/merge-patch+json`      | RFC 7386 — JSON object; `null` deletes a field            |
| `application/merge-patch+shorthand` | Shorthand DSL (Daniel G. Taylor) overlaid on the original |
| `application/json` / empty          | Compatibility: treated as Merge Patch                     |

Unsupported media types return `415 Unsupported Media Type` (`PATCH_UNSUPPORTED_MEDIA_TYPE`).

The implementation lives in `pkg/patch` (wrapped by `decodePatch` in `pkg/handlers/helpers.go`). Every domain's PATCH handler follows the same flow:

```go
func (h *BookHandler) Patch(c *echo.Context) error {
    ctx := c.Request().Context()
    var path types.BookPath
    if err := echo.BindPathValues(c, &path); err != nil {
        return err
    }

    current, err := h.book.Get(ctx, path.Book)
    if err != nil {
        return err
    }

    req, err := decodePatch[types.ReplaceBookRequest](c, toBook(current))
    if err != nil {
        return err
    }
    req.Book = path.Book               // path is authoritative
    if err := req.Validate(); err != nil {
        return err
    }
    b, err := h.book.Replace(ctx, path.Book, &req)
    if err != nil {
        return err
    }
    return c.JSON(http.StatusOK, toBook(b))
}
```

Rules:
- PATCH handlers **must** call `echo.BindPathValues` directly — `c.Bind` would try to parse the body, which may legitimately be a JSON array (RFC 6902) or non-JSON shorthand.
- After unmarshalling the patched JSON into the `ReplaceXxxRequest`, **always** reassign the path-derived ID. Never trust the patched body for identity.
- PATCH and PUT must share the service-layer `Replace` method, so all three patch protocols and the explicit PUT path get the same validation and persistence semantics.
- Read-only fields (`id`, `created_at`, `updated_at`, etc.) are absent from `ReplaceXxxRequest`, so patches that attempt to mutate them are dropped silently during unmarshalling.

### Request & Response struct conventions

Every handler binds via a dedicated struct. **Never call `c.Param` directly.**

| Method   | Request struct                                                        | Response struct               |
|----------|-----------------------------------------------------------------------|-------------------------------|
| POST     | `CreateXxxRequest{ XxxID string \`json:"xxx_id,omitempty"\`, ... }`   | `types.Xxx` (201)             |
| GET (1)  | `XxxPath{ Xxx string \`param:"xxx"\` }`                               | `types.Xxx` (200)             |
| PUT      | `ReplaceXxxRequest{ Xxx string \`param:"xxx" json:"-"\`, ... }`       | `types.Xxx` (200)             |
| PATCH    | `XxxPath{ Xxx string \`param:"xxx"\` }` + `decodePatch`               | `types.Xxx` (200)             |
| DELETE   | `XxxPath{ Xxx string \`param:"xxx"\` }`                               | 204 No Content                |
| LIST     | `types.ListRequest` (or domain-specific list request)                 | `types.XxxList` (200)         |

### Create methods

- `CreateXxxRequest` **should** include an optional `xxx_id` field (ULID format). If provided it becomes the resource ID; if absent, generate via `utils.GenerateULID()`.
- Register an `XXX_ALREADY_EXISTS` error code mapped to `http.StatusConflict` (409); return it on `ent.IsConstraintError`.

### Replace methods (PUT)

- `ReplaceXxxRequest` contains every writable field. Required fields are validated by `Validate()`; optional fields that come in as zero value are explicitly **cleared** in the service.
- `id`, `created_at`, `updated_at` are not part of `ReplaceXxxRequest`. They cannot be modified via PUT or PATCH.

### Delete methods

- Bind `XxxPath{ Xxx string \`param:"xxx"\` }` — no request body.
- Return `404` (`XXX_NOT_FOUND`) if resource does not exist. Response is `204 No Content`.

### List methods

Every `List` handler binds `types.ListRequest` (or a resource-specific struct if extra query params are needed) via `bindAndValidate` and returns a flat `types.XxxList` (`{"items": [...], "page", "page_size", "total_size"}`).

- **Pagination**: `page int32` (1-based, default 1, reject if negative) + `page_size int32` (default 50, max 1000; coerce if over max, reject if negative). Use `resolveListOptions` in `pkg/handlers/pagination.go` to compute `metav1.ListOptions{Offset, Limit, OrderBy, Filter}` and normalized `page`.
- **Ordering**: `order_by string` — comma-separated `"field"` or `"field desc"`. Document supported fields. Default sort must be defined. Return `*validator.FieldViolations` (using `validator.KeyInvalidOrderBy*` / `validator.KeyUnknownOrderByField`) on errors.
- **Filtering**: `filter string` — call `filter.Parse(in.Filter)` → `Expr` AST, translate with `translateXxxFilter(expr)` in `pkg/services/<domain>_filter.go` using `filter.FilterTranslator[P]`. Return `ValidationInvalid` on errors; never build raw SQL. See `pkg/services/book_filter.go` as reference.
- Service returns `([]*ent.Xxx, totalSize int32, error)`. Handler constructs `types.XxxList{Items: mapSlice(items, toXxx), Page: page, PageSize: opts.Limit, TotalSize: total}`.

### Custom methods (resource actions)

Verbs are kebab-case path segments after the resource.

- Examples: `POST /api/v1/cabinets/:cabinet/approve`, `POST /api/v1/cabinets/:cabinet/reject`, `POST /api/v1/users/:user/reset-password`, `POST /api/v1/books/:book/archive`.
- Collection-level verbs go directly under the collection: `POST /api/v1/books/batch-get`.
- HTTP method: idempotent state transitions use `PUT`; non-idempotent / side-effect actions use `POST`. Read-only sub-state queries use `GET`.
- Handler method is named after the verb in PascalCase (`Approve`, `Reject`, `ResetPassword`, `Archive`); register with the matching path: `g.POST("/:cabinet/approve", h.Approve)`.
- Error codes for verb actions follow `<DOMAIN>_<VERB>_<REASON>` when the failure is verb-specific, e.g. `CABINET_APPROVE_FORBIDDEN`. Otherwise reuse existing domain codes.

---

## Naming

- Constructors: `NewFooService(i do.Injector) (*FooService, error)` — always accept injector.
- Error codes: `SCREAMING_SNAKE_CASE` string constants, grouped by domain in `pkg/codes/<domain>.go`.
- Handler methods: match HTTP semantics — `Create`, `List`, `Get`, `Replace`, `Patch`, `Delete`. Custom verbs use PascalCase of the verb (`Archive`, `ResetPassword`).
- Service methods: same as handler methods (`Create`, `Get`, `List`, `Replace`, `Delete`, …). PATCH does not appear in the service layer — it composes `Get` + `Replace` in the handler.
- Internal helpers: unexported, placed at bottom of file.
- Route path parameters: name the path variable after the resource type in singular form — e.g. `/:book`, `/:publisher`. Never use generic `/:id` or `/:name`.

---

## Forbidden Patterns

| Pattern | Reason |
|---|---|
| `fmt.Errorf` in service layer | Breaks error classification pipeline |
| Returning `*ent.Xxx` from handler | Leaks internal fields |
| `bcrypt` calls outside `pkg/utils` | Centralised cost policy (cost=12) |
| Registering duplicate error codes | `init()` panics by design |
| Storing `context.Context` in struct | Violates Go context contract |
| Adding user data to JWT claims | Over-exposure; trust DB as source of truth |
| Manually editing `static/openapi/openapi.yaml` | Generated by `make swag`; manual edits are overwritten |
| `c.Param(...)` called directly in handler | Use typed `XxxPath{ Xxx string \`param:"xxx"\` }` and `c.Bind` or `echo.BindPathValues` |
| `c.Bind` in a PATCH handler | Body may be a JSON Patch array; use `echo.BindPathValues` + `decodePatch` instead |
| Skipping `req.Xxx = path.Xxx` after patch | Patches can fake the path-derived ID; the URL must remain authoritative |
| Re-implementing PATCH instead of calling `Replace` | All three patch protocols must converge on the service-layer `Replace` |
| `:verb`-suffix routes (e.g. `/:book:archive`) | Echo router scans `:` greedily up to the next `/`, so `book:archive` becomes a single param name. Use sub-path verb form `/:book/archive` |
| camelCase verb segments (e.g. `/resetPassword`) | Inconsistent with the rest of the URL space; use kebab-case (`/reset-password`) |
| Raw `http.Get`/`http.Post` outside `pkg/clients/` | Bypasses shared transport, retry, and error classification |
| Depending on `*Client` concrete type in services | Use the `API` interface; keeps services testable |
