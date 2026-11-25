import { getDoc } from "@typespec/compiler";
import { getOperationId } from "@typespec/openapi";
import { baseScalarName, toGoName } from "./go-types.mjs";

/**
 * Derive the Go method name directly from the TypeSpec operation name (1:1 PascalCase mapping):
 *   Books.createBook -> CreateBook, Books.listBooks -> ListBooks, Me.getMe -> GetMe, Auth.login -> Login
 */
export function operationMethodName(opName) {
  return toGoName(opName);
}

/** Extract success status code and optional response model name from an HttpOperation. */
export function resolveSuccessResponse(httpOp) {
  for (const resp of httpOp.responses) {
    const code = typeof resp.statusCodes === "number" ? resp.statusCodes : 200;
    if (code >= 200 && code < 300) {
      for (const r of resp.responses) {
        if (r.body && r.body.type && r.body.type.kind === "Model" && r.body.type.name) {
          return { status: code, modelName: r.body.type.name };
        }
      }
      return { status: code, modelName: null };
    }
  }
  return { status: 200, modelName: null };
}

/**
 * Extract TypeSpec @useAuth requirements as OpenAPI-style DNF:
 *   - []                              => public (no auth or NoAuth only)
 *   - [["BearerAuth"]]                => BearerAuth
 *   - [["BearerAuth"], ["ApiKeyAuth"]]=> BearerAuth OR ApiKeyAuth
 *   - [["BearerAuth", "ApiKeyAuth"]]  => BearerAuth AND ApiKeyAuth
 *   - [["BearerAuth"], []]            => BearerAuth OR anonymous (NoAuth placed last)
 */
export function resolveAuthOptions(httpOp) {
  const auth = httpOp.authentication;
  if (!auth || !auth.options || auth.options.length === 0) {
    return [];
  }
  const nonEmpty = [];
  let hasNoAuth = false;
  for (const opt of auth.options) {
    const ids = (opt.schemes || [])
      .filter((s) => s.type !== "noAuth" && s.id)
      .map((s) => s.id);
    if (ids.length === 0) {
      hasNoAuth = true;
    } else {
      nonEmpty.push(ids);
    }
  }
  if (nonEmpty.length === 0) {
    return [];
  }
  if (hasNoAuth) {
    nonEmpty.push([]);
  }
  return nonEmpty;
}

function formatOperationLiteral(opID, method, path, authOptions) {
  const fields = [
    `ID: ${JSON.stringify(opID)}`,
    `Method: ${httpMethodConst(method)}`,
    `Pattern: ${JSON.stringify(path)}`,
  ];
  if (authOptions && authOptions.length > 0) {
    const alts = authOptions
      .map((alt) => `{${alt.map((s) => JSON.stringify(s)).join(", ")}}`)
      .join(", ");
    fields.push(`Auth: [][]string{${alts}}`);
  }
  return `rest.Operation{${fields.join(", ")}}`;
}

function httpMethodConst(method) {
  switch (method) {
    case "GET":
      return "http.MethodGet";
    case "POST":
      return "http.MethodPost";
    case "PUT":
      return "http.MethodPut";
    case "PATCH":
      return "http.MethodPatch";
    case "DELETE":
      return "http.MethodDelete";
    default:
      return JSON.stringify(method);
  }
}

/**
 * Emit the Connect-Go v2 style handler interface and Go 1.22+ net/http.ServeMux mount function.
 */
export function emitInterfaceAndMount(program, ifaceName, opInfos) {
  const handlerName = `${ifaceName}Handler`;
  const lines = [];

  // 1. Pure Go Handler interface (excluding @autoPatch operations).
  lines.push(`// ${handlerName} 由 TypeSpec 自动生成（Connect-Go v2 风格纯业务接口）。`);
  lines.push(`// Handler 层只需实现此接口，负责编排 Service 与 DTO 投影，无需接触 HTTP 协议细节。`);
  lines.push(`type ${handlerName} interface {`);
  for (const info of opInfos) {
    if (info.autoPatch) continue;
    const doc = getDoc(program, info.httpOp.operation);
    if (doc) {
      lines.push(`\t// ${info.methodName} ${doc}`);
    }
    const paramsSig = info.reqStruct
      ? `ctx context.Context, req *${info.reqStruct}`
      : `ctx context.Context`;
    const returnSig = info.respModel
      ? `(*${info.respModel}, error)`
      : `error`;
    lines.push(`\t${info.methodName}(${paramsSig}) ${returnSig}`);
  }
  lines.push("}");
  lines.push("");

  // 2. Connect-Go style constructor returning rest.Handler (implements http.Handler + Mount(*http.ServeMux)).
  lines.push(`// New${handlerName} 构造符合 Connect-Go 风格的 rest.Handler（同时实现标准 http.Handler）。`);
  lines.push(`// 可直接挂载到 rest.Router（router.Mount(api.New${handlerName}(impl, ...))），也可独立用于 httptest。`);
  lines.push(`func New${handlerName}(impl ${handlerName}, opts ...rest.HandlerOption) rest.Handler {`);
  lines.push(`\tcfg := rest.ApplyHandlerOptions(opts...)`);
  lines.push(`\treturn rest.NewHandler(func(mux *http.ServeMux) {`);

  const opByRef = new Map(opInfos.map((info) => [info.httpOp.operation, info]));

  for (const info of opInfos) {
    const verbUpper = info.httpOp.verb.toUpperCase();
    const pattern = `${verbUpper} ${info.httpOp.path}`;
    const opID = getOperationId(program, info.httpOp.operation) || info.httpOp.operation.name;
    const opLiteral = formatOperationLiteral(opID, verbUpper, info.httpOp.path, info.authOptions);
    lines.push(`\t\tmux.Handle(${JSON.stringify(pattern)}, cfg.Wrap(${opLiteral}, func(w http.ResponseWriter, r *http.Request) {`);
    lines.push(`\t\t\tctx := r.Context()`);

    if (info.autoPatch) {
      const getInfo = opByRef.get(info.autoPatch.getOp);
      const replaceInfo = opByRef.get(info.autoPatch.replaceOp);
      lines.push(...emitAutoPatchBody(info, getInfo, replaceInfo));
    } else {
      lines.push(...emitStandardOpBody(info));
    }

    lines.push(`\t\t}))`);
    lines.push("");
  }

  lines.push(`\t})`);
  lines.push("}");
  return lines.join("\n");
}

function emitStandardOpBody(info) {
  const lines = [];

  if (info.reqStruct) {
    lines.push(`\t\tvar req ${info.reqStruct}`);

    if (info.bodyModel) {
      lines.push(`\t\tif err := rest.DecodeJSON(r, &req); err != nil {`);
      lines.push(`\t\t\tcfg.WriteError(w, r, err)`);
      lines.push(`\t\t\treturn`);
      lines.push(`\t\t}`);
    }

    for (const p of info.pathParams) {
      const expr = p.param.type.kind === "Enum"
        ? `${p.param.type.name}(r.PathValue(${JSON.stringify(p.name)}))`
        : `r.PathValue(${JSON.stringify(p.name)})`;
      lines.push(`\t\treq.${toGoName(p.name)} = ${expr}`);
    }

    if (info.queryParams.length > 0) {
      lines.push(`\t\tq := r.URL.Query()`);
      const hasTypedQuery = info.queryParams.some((p) => {
        const base = p.param.type.kind === "Scalar" ? baseScalarName(p.param.type) : "";
        return base === "int32" || base === "int64" || base === "float64" || base === "boolean";
      });
      if (hasTypedQuery) {
        lines.push(`\t\tvar qv validator.FieldViolations`);
      }
      for (const p of info.queryParams) {
        const goField = toGoName(p.name);
        if (p.param.type.kind === "Enum") {
          lines.push(`\t\treq.${goField} = ${p.param.type.name}(q.Get(${JSON.stringify(p.name)}))`);
          continue;
        }
        const scalarName = p.param.type.kind === "Scalar" ? baseScalarName(p.param.type) : "string";
        if (scalarName === "int32") {
          lines.push(`\t\treq.${goField} = rest.ParseInt32Query(q, ${JSON.stringify(p.name)}, &qv)`);
        } else if (scalarName === "int64") {
          lines.push(`\t\treq.${goField} = rest.ParseInt64Query(q, ${JSON.stringify(p.name)}, &qv)`);
        } else if (scalarName === "float64") {
          const fn = p.param.optional ? "ParseOptionalFloat64Query" : "ParseFloat64Query";
          lines.push(`\t\treq.${goField} = rest.${fn}(q, ${JSON.stringify(p.name)}, &qv)`);
        } else if (scalarName === "boolean") {
          const fn = p.param.optional ? "ParseOptionalBoolQuery" : "ParseBoolQuery";
          lines.push(`\t\treq.${goField} = rest.${fn}(q, ${JSON.stringify(p.name)}, &qv)`);
        } else {
          lines.push(`\t\treq.${goField} = q.Get(${JSON.stringify(p.name)})`);
        }
      }
      if (hasTypedQuery) {
        lines.push(`\t\tif err := qv.Err(); err != nil {`);
        lines.push(`\t\t\tcfg.WriteError(w, r, err)`);
        lines.push(`\t\t\treturn`);
        lines.push(`\t\t}`);
      }
    }

    if (info.hasValidate) {
      lines.push(`\t\tif err := req.Validate(); err != nil {`);
      lines.push(`\t\t\tcfg.WriteError(w, r, err)`);
      lines.push(`\t\t\treturn`);
      lines.push(`\t\t}`);
    }
  }

  const callArgs = info.reqStruct ? "ctx, &req" : "ctx";
  if (info.respModel) {
    lines.push(`\t\tres, err := impl.${info.methodName}(${callArgs})`);
    lines.push(`\t\tif err != nil {`);
    lines.push(`\t\t\tcfg.WriteError(w, r, err)`);
    lines.push(`\t\t\treturn`);
    lines.push(`\t\t}`);
    lines.push(`\t\trest.WriteJSON(w, r, ${httpStatusConst(info.status)}, res)`);
  } else {
    lines.push(`\t\tif err := impl.${info.methodName}(${callArgs}); err != nil {`);
    lines.push(`\t\t\tcfg.WriteError(w, r, err)`);
    lines.push(`\t\t\treturn`);
    lines.push(`\t\t}`);
    if (info.status === 204) {
      lines.push(`\t\trest.WriteNoContent(w)`);
    } else {
      lines.push(`\t\tw.WriteHeader(${httpStatusConst(info.status)})`);
    }
  }
  return lines;
}

function emitAutoPatchBody(info, getInfo, replaceInfo) {
  const lines = [];
  lines.push(`\t\t// Auto-generated by @autoPatch(${getInfo.httpOp.operation.name}, ${replaceInfo.httpOp.operation.name})`);

  if (getInfo.reqStruct) {
    lines.push(`\t\tvar getReq ${getInfo.reqStruct}`);
    for (const p of info.pathParams) {
      lines.push(`\t\tgetReq.${toGoName(p.name)} = r.PathValue(${JSON.stringify(p.name)})`);
    }
    if (getInfo.hasValidate) {
      lines.push(`\t\tif err := getReq.Validate(); err != nil {`);
      lines.push(`\t\t\tcfg.WriteError(w, r, err)`);
      lines.push(`\t\t\treturn`);
      lines.push(`\t\t}`);
    }
    lines.push(`\t\tcurrent, err := impl.${getInfo.methodName}(ctx, &getReq)`);
  } else {
    lines.push(`\t\tcurrent, err := impl.${getInfo.methodName}(ctx)`);
  }

  lines.push(`\t\tif err != nil {`);
  lines.push(`\t\t\tcfg.WriteError(w, r, err)`);
  lines.push(`\t\t\treturn`);
  lines.push(`\t\t}`);
  lines.push(`\t\tvar req ${replaceInfo.reqStruct}`);
  lines.push(`\t\tif err := rest.ApplyPatch(r, current, &req); err != nil {`);
  lines.push(`\t\t\tcfg.WriteError(w, r, err)`);
  lines.push(`\t\t\treturn`);
  lines.push(`\t\t}`);
  for (const p of info.pathParams) {
    lines.push(`\t\treq.${toGoName(p.name)} = r.PathValue(${JSON.stringify(p.name)})`);
  }
  if (replaceInfo.hasValidate) {
    lines.push(`\t\tif err := req.Validate(); err != nil {`);
    lines.push(`\t\t\tcfg.WriteError(w, r, err)`);
    lines.push(`\t\t\treturn`);
    lines.push(`\t\t}`);
  }
  lines.push(`\t\tres, err := impl.${replaceInfo.methodName}(ctx, &req)`);
  lines.push(`\t\tif err != nil {`);
  lines.push(`\t\t\tcfg.WriteError(w, r, err)`);
  lines.push(`\t\t\treturn`);
  lines.push(`\t\t}`);
  lines.push(`\t\trest.WriteJSON(w, r, http.StatusOK, res)`);
  return lines;
}

function httpStatusConst(code) {
  switch (code) {
    case 200:
      return "http.StatusOK";
    case 201:
      return "http.StatusCreated";
    case 202:
      return "http.StatusAccepted";
    case 204:
      return "http.StatusNoContent";
    default:
      return String(code);
  }
}
