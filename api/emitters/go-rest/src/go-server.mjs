import { getDoc } from "@typespec/compiler";
import { getOperationId } from "@typespec/openapi";
import { baseScalarName, isOptionalPointer, stringLiteralValues, toGoName } from "./go-types.mjs";
import { reportDiagnostic } from "./lib.mjs";

export function operationMethodName(opName) {
  return toGoName(opName);
}

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

export function needsTypedQueryValidator(queryParams) {
  return queryParams.some((p) => {
    if (p.param.type.kind !== "Scalar") return false;
    const base = baseScalarName(p.param.type);
    return base === "int32" || base === "int64" || base === "float64" || base === "boolean" || base === "utcDateTime" || base === "offsetDateTime";
  });
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
  return `httpkit.Operation{${fields.join(", ")}}`;
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

function pathValueExpr(p) {
  const raw = `r.PathValue(${JSON.stringify(p.name)})`;
  if (p.param.type.kind === "Enum") {
    return `${p.param.type.name}(${raw})`;
  }
  return raw;
}

export function emitInterfaceAndMount(program, ifaceName, opInfos) {
  const handlerName = `${ifaceName}Handler`;
  const lines = [];

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

  lines.push(`func New${handlerName}(impl ${handlerName}, opts ...httpkit.HandlerOption) httpkit.Handler {`);
  lines.push(`\tcfg := httpkit.ApplyHandlerOptions(opts...)`);
  lines.push(`\treturn httpkit.NewHandler(func(mux *http.ServeMux) {`);

  const opByRef = new Map(opInfos.map((info) => [info.httpOp.operation, info]));

  for (const info of opInfos) {
    const verbUpper = info.httpOp.verb.toUpperCase();
    const pattern = `${verbUpper} ${info.httpOp.path}`;
    const opID = getOperationId(program, info.httpOp.operation) || info.httpOp.operation.name;
    const opLiteral = formatOperationLiteral(opID, verbUpper, info.httpOp.path, info.authOptions);

    let bodyLines;
    if (info.autoPatch) {
      const getInfo = opByRef.get(info.autoPatch.getOp);
      const replaceInfo = opByRef.get(info.autoPatch.replaceOp);
      if (!getInfo || !replaceInfo || !replaceInfo.reqStruct) {
        reportDiagnostic(program, {
          code: "invalid-auto-patch",
          format: { op: opID },
          target: info.httpOp.operation,
        });
        continue;
      }
      bodyLines = emitAutoPatchBody(info, getInfo, replaceInfo);
    } else {
      bodyLines = emitStandardOpBody(program, opID, info);
    }

    lines.push(`\t\tmux.Handle(${JSON.stringify(pattern)}, cfg.Wrap(${opLiteral}, func(w http.ResponseWriter, r *http.Request) {`);
    lines.push(`\t\t\tctx := r.Context()`);
    lines.push(...bodyLines);
    lines.push(`\t\t}))`);
    lines.push("");
  }

  lines.push(`\t})`);
  lines.push("}");
  return lines.join("\n");
}

function emitStandardOpBody(program, opID, info) {
  const lines = [];

  if (info.reqStruct) {
    lines.push(`\t\t\tvar req ${info.reqStruct}`);

    if (info.bodyModel) {
      lines.push(`\t\t\tif err := httpkit.DecodeJSON(r, &req); err != nil {`);
      lines.push(`\t\t\t\tcfg.WriteError(w, r, err)`);
      lines.push(`\t\t\t\treturn`);
      lines.push(`\t\t\t}`);
    }

    for (const p of info.pathParams) {
      lines.push(`\t\t\treq.${toGoName(p.name)} = ${pathValueExpr(p)}`);
    }

    if (info.queryParams.length > 0) {
      lines.push(`\t\t\tq := r.URL.Query()`);
      const hasTypedQuery = needsTypedQueryValidator(info.queryParams);
      if (hasTypedQuery) {
        lines.push(`\t\t\tvar qv validator.FieldViolations`);
      }
      for (const p of info.queryParams) {
        emitQueryParamDecode(program, opID, p, lines);
      }
      if (hasTypedQuery) {
        lines.push(`\t\t\tif err := qv.Err(); err != nil {`);
        lines.push(`\t\t\t\tcfg.WriteError(w, r, err)`);
        lines.push(`\t\t\t\treturn`);
        lines.push(`\t\t\t}`);
      }
    }

    if (info.hasValidate) {
      lines.push(`\t\t\tif err := req.Validate(); err != nil {`);
      lines.push(`\t\t\t\tcfg.WriteError(w, r, err)`);
      lines.push(`\t\t\t\treturn`);
      lines.push(`\t\t\t}`);
    }
  }

  const callArgs = info.reqStruct ? "ctx, &req" : "ctx";
  if (info.respModel) {
    lines.push(`\t\t\tres, err := impl.${info.methodName}(${callArgs})`);
    lines.push(`\t\t\tif err != nil {`);
    lines.push(`\t\t\t\tcfg.WriteError(w, r, err)`);
    lines.push(`\t\t\t\treturn`);
    lines.push(`\t\t\t}`);
    lines.push(`\t\t\thttpkit.WriteJSON(w, r, ${httpStatusConst(info.status)}, res)`);
  } else {
    lines.push(`\t\t\tif err := impl.${info.methodName}(${callArgs}); err != nil {`);
    lines.push(`\t\t\t\tcfg.WriteError(w, r, err)`);
    lines.push(`\t\t\t\treturn`);
    lines.push(`\t\t\t}`);
    if (info.status === 204) {
      lines.push(`\t\t\thttpkit.WriteNoContent(w)`);
    } else {
      lines.push(`\t\t\tw.WriteHeader(${httpStatusConst(info.status)})`);
    }
  }
  return lines;
}

function emitQueryParamDecode(program, opID, p, lines) {
  const goField = toGoName(p.name);
  const nameLit = JSON.stringify(p.name);
  const paramType = p.param.type;
  const optPtr = isOptionalPointer(p.param);

  if (paramType.kind === "Enum") {
    lines.push(`\t\t\treq.${goField} = ${paramType.name}(q.Get(${nameLit}))`);
    return;
  }

  if (paramType.kind === "Union" && stringLiteralValues(paramType)) {
    lines.push(`\t\t\treq.${goField} = q.Get(${nameLit})`);
    return;
  }

  if (paramType.kind === "Model" && paramType.name === "Array" && paramType.indexer) {
    const elem = paramType.indexer.value;
    if (elem.kind === "Enum") {
      lines.push(`\t\t\tif rawItems := httpkit.ParseStringSliceQuery(q, ${nameLit}); len(rawItems) > 0 {`);
      lines.push(`\t\t\t\treq.${goField} = make([]${elem.name}, len(rawItems))`);
      lines.push(`\t\t\t\tfor i, s := range rawItems {`);
      lines.push(`\t\t\t\t\treq.${goField}[i] = ${elem.name}(s)`);
      lines.push(`\t\t\t\t}`);
      lines.push(`\t\t\t}`);
      return;
    }
    if (elem.kind === "Scalar" && baseScalarName(elem) === "string") {
      lines.push(`\t\t\treq.${goField} = httpkit.ParseStringSliceQuery(q, ${nameLit})`);
      return;
    }
  }

  if (paramType.kind === "Scalar") {
    const scalarName = baseScalarName(paramType);
    switch (scalarName) {
      case "string":
        lines.push(`\t\t\treq.${goField} = q.Get(${nameLit})`);
        return;
      case "int32": {
        const fn = optPtr ? "ParseOptionalInt32Query" : "ParseInt32Query";
        lines.push(`\t\t\treq.${goField} = httpkit.${fn}(q, ${nameLit}, &qv)`);
        return;
      }
      case "int64": {
        const fn = optPtr ? "ParseOptionalInt64Query" : "ParseInt64Query";
        lines.push(`\t\t\treq.${goField} = httpkit.${fn}(q, ${nameLit}, &qv)`);
        return;
      }
      case "float64": {
        const fn = optPtr ? "ParseOptionalFloat64Query" : "ParseFloat64Query";
        lines.push(`\t\t\treq.${goField} = httpkit.${fn}(q, ${nameLit}, &qv)`);
        return;
      }
      case "boolean": {
        const fn = optPtr ? "ParseOptionalBoolQuery" : "ParseBoolQuery";
        lines.push(`\t\t\treq.${goField} = httpkit.${fn}(q, ${nameLit}, &qv)`);
        return;
      }
      case "utcDateTime":
      case "offsetDateTime": {
        const fn = optPtr ? "ParseOptionalTimeQuery" : "ParseTimeQuery";
        lines.push(`\t\t\treq.${goField} = httpkit.${fn}(q, ${nameLit}, &qv)`);
        return;
      }
    }
  }

  reportDiagnostic(program, {
    code: "unsupported-query-type",
    format: { param: p.name, op: opID, type: paramType.name || paramType.kind },
    target: p.param,
  });
}

function emitAutoPatchBody(info, getInfo, replaceInfo) {
  const lines = [];

  // 先校验 Content-Type 与补丁 JSON 合法性再查库，避免非法请求触发无意义的 DB 查询。
  lines.push(`\t\t\tpatchDoc, err := httpkit.ParsePatchRequest(r)`);
  lines.push(`\t\t\tif err != nil {`);
  lines.push(`\t\t\t\tcfg.WriteError(w, r, err)`);
  lines.push(`\t\t\t\treturn`);
  lines.push(`\t\t\t}`);

  if (getInfo.reqStruct) {
    lines.push(`\t\t\tvar getReq ${getInfo.reqStruct}`);
    for (const p of info.pathParams) {
      lines.push(`\t\t\tgetReq.${toGoName(p.name)} = ${pathValueExpr(p)}`);
    }
    if (getInfo.hasValidate) {
      lines.push(`\t\t\tif err := getReq.Validate(); err != nil {`);
      lines.push(`\t\t\t\tcfg.WriteError(w, r, err)`);
      lines.push(`\t\t\t\treturn`);
      lines.push(`\t\t\t}`);
    }
    lines.push(`\t\t\tcurrent, err := impl.${getInfo.methodName}(ctx, &getReq)`);
  } else {
    lines.push(`\t\t\tcurrent, err := impl.${getInfo.methodName}(ctx)`);
  }

  lines.push(`\t\t\tif err != nil {`);
  lines.push(`\t\t\t\tcfg.WriteError(w, r, err)`);
  lines.push(`\t\t\t\treturn`);
  lines.push(`\t\t\t}`);
  lines.push(`\t\t\tvar req ${replaceInfo.reqStruct}`);
  lines.push(`\t\t\tif err := patchDoc.Apply(current, &req); err != nil {`);
  lines.push(`\t\t\t\tcfg.WriteError(w, r, err)`);
  lines.push(`\t\t\t\treturn`);
  lines.push(`\t\t\t}`);
  for (const p of info.pathParams) {
    lines.push(`\t\t\treq.${toGoName(p.name)} = ${pathValueExpr(p)}`);
  }
  if (replaceInfo.hasValidate) {
    lines.push(`\t\t\tif err := req.Validate(); err != nil {`);
    lines.push(`\t\t\t\tcfg.WriteError(w, r, err)`);
    lines.push(`\t\t\t\treturn`);
    lines.push(`\t\t\t}`);
  }
  lines.push(`\t\t\tres, err := impl.${replaceInfo.methodName}(ctx, &req)`);
  lines.push(`\t\t\tif err != nil {`);
  lines.push(`\t\t\t\tcfg.WriteError(w, r, err)`);
  lines.push(`\t\t\t\treturn`);
  lines.push(`\t\t\t}`);
  lines.push(`\t\t\thttpkit.WriteJSON(w, r, ${httpStatusConst(info.status)}, res)`);
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
