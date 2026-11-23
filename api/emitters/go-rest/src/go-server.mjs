import { getDoc } from "@typespec/compiler";
import { toGoName } from "./go-types.mjs";

const CRUD_VERBS = new Set(["create", "get", "replace", "delete", "list"]);

/**
 * Derive idiomatic Go method name for an operation in an interface:
 *   - Standard CRUD verbs append the resource noun: Books.create -> CreateBook, Books.list -> ListBooks, Me.get -> GetMe
 *   - Custom action verbs use the verb directly: Auth.login -> Login, Auth.refreshToken -> RefreshToken, Me.changePassword -> ChangePassword
 */
export function operationMethodName(ifaceName, opName) {
  const verb = toGoName(opName);
  if (!CRUD_VERBS.has(opName)) {
    return verb;
  }
  if (opName === "list") {
    return `${verb}${ifaceName}`;
  }
  const singular = ifaceName.endsWith("s") ? ifaceName.slice(0, -1) : ifaceName;
  return `${verb}${singular}`;
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

/** Check whether an HttpOperation requires authentication (@useAuth). */
export function requiresAuth(httpOp) {
  const auth = httpOp.authentication;
  return Boolean(auth && auth.options && auth.options.some((o) => o.schemes && o.schemes.length > 0));
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

  // 2. Mount function targeting Go 1.22+ net/http.ServeMux.
  lines.push(`// Mount${handlerName} 将 ${handlerName} 挂载到标准 *http.ServeMux。`);
  lines.push(`// 自动处理路由注册、鉴权包装、零反射参数解码、Validate()、@autoPatch 适配以及 RFC 9457 错误响应。`);
  lines.push(`func Mount${handlerName}(mux *http.ServeMux, impl ${handlerName}, rt *httpruntime.Runtime, auth func(http.Handler) http.Handler) {`);
  lines.push(`\twrap := func(needAuth bool, fn http.HandlerFunc) http.Handler {`);
  lines.push(`\t\tvar h http.Handler = fn`);
  lines.push(`\t\tif needAuth && auth != nil {`);
  lines.push(`\t\t\th = auth(h)`);
  lines.push(`\t\t}`);
  lines.push(`\t\treturn h`);
  lines.push(`\t}`);
  lines.push("");

  const opByRef = new Map(opInfos.map((info) => [info.httpOp.operation, info]));

  for (const info of opInfos) {
    const pattern = `${info.httpOp.verb.toUpperCase()} ${info.httpOp.path}`;
    lines.push(`\tmux.Handle(${JSON.stringify(pattern)}, wrap(${info.needAuth}, func(w http.ResponseWriter, r *http.Request) {`);
    lines.push(`\t\tctx := r.Context()`);

    if (info.autoPatch) {
      const getInfo = opByRef.get(info.autoPatch.getOp);
      const replaceInfo = opByRef.get(info.autoPatch.replaceOp);
      lines.push(...emitAutoPatchBody(info, getInfo, replaceInfo));
    } else {
      lines.push(...emitStandardOpBody(info));
    }

    lines.push(`\t}))`);
    lines.push("");
  }

  lines.push("}");
  return lines.join("\n");
}

function emitStandardOpBody(info) {
  const lines = [];

  if (info.reqStruct) {
    lines.push(`\t\tvar req ${info.reqStruct}`);

    if (info.bodyModel) {
      lines.push(`\t\tif err := rt.DecodeJSON(r, &req); err != nil {`);
      lines.push(`\t\t\trt.WriteProblem(w, r, err)`);
      lines.push(`\t\t\treturn`);
      lines.push(`\t\t}`);
    }

    for (const p of info.pathParams) {
      lines.push(`\t\treq.${toGoName(p.name)} = r.PathValue(${JSON.stringify(p.name)})`);
    }

    if (info.queryParams.length > 0) {
      lines.push(`\t\tq := r.URL.Query()`);
      const hasIntQuery = info.queryParams.some((p) => {
        const base = p.param.type.kind === "Scalar" ? p.param.type.name : "";
        return base === "int32" || base === "int64";
      });
      if (hasIntQuery) {
        lines.push(`\t\tvar qv validator.FieldViolations`);
      }
      for (const p of info.queryParams) {
        const goField = toGoName(p.name);
        const scalarName = p.param.type.kind === "Scalar" ? p.param.type.name : "string";
        if (scalarName === "int32") {
          lines.push(`\t\treq.${goField} = rt.ParseInt32Query(q, ${JSON.stringify(p.name)}, &qv)`);
        } else {
          lines.push(`\t\treq.${goField} = q.Get(${JSON.stringify(p.name)})`);
        }
      }
      if (hasIntQuery) {
        lines.push(`\t\tif err := qv.Err(); err != nil {`);
        lines.push(`\t\t\trt.WriteProblem(w, r, err)`);
        lines.push(`\t\t\treturn`);
        lines.push(`\t\t}`);
      }
    }

    if (info.hasValidate) {
      lines.push(`\t\tif err := req.Validate(); err != nil {`);
      lines.push(`\t\t\trt.WriteProblem(w, r, err)`);
      lines.push(`\t\t\treturn`);
      lines.push(`\t\t}`);
    }
  }

  const callArgs = info.reqStruct ? "ctx, &req" : "ctx";
  if (info.respModel) {
    lines.push(`\t\tres, err := impl.${info.methodName}(${callArgs})`);
    lines.push(`\t\tif err != nil {`);
    lines.push(`\t\t\trt.WriteProblem(w, r, err)`);
    lines.push(`\t\t\treturn`);
    lines.push(`\t\t}`);
    lines.push(`\t\trt.WriteJSON(w, ${httpStatusConst(info.status)}, res)`);
  } else {
    lines.push(`\t\tif err := impl.${info.methodName}(${callArgs}); err != nil {`);
    lines.push(`\t\t\trt.WriteProblem(w, r, err)`);
    lines.push(`\t\t\treturn`);
    lines.push(`\t\t}`);
    lines.push(`\t\trt.WriteNoContent(w)`);
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
      lines.push(`\t\t\trt.WriteProblem(w, r, err)`);
      lines.push(`\t\t\treturn`);
      lines.push(`\t\t}`);
    }
    lines.push(`\t\tcurrent, err := impl.${getInfo.methodName}(ctx, &getReq)`);
  } else {
    lines.push(`\t\tcurrent, err := impl.${getInfo.methodName}(ctx)`);
  }

  lines.push(`\t\tif err != nil {`);
  lines.push(`\t\t\trt.WriteProblem(w, r, err)`);
  lines.push(`\t\t\treturn`);
  lines.push(`\t\t}`);
  lines.push(`\t\tvar req ${replaceInfo.reqStruct}`);
  lines.push(`\t\tif err := rt.ApplyPatch(r, current, &req); err != nil {`);
  lines.push(`\t\t\trt.WriteProblem(w, r, err)`);
  lines.push(`\t\t\treturn`);
  lines.push(`\t\t}`);
  for (const p of info.pathParams) {
    lines.push(`\t\treq.${toGoName(p.name)} = r.PathValue(${JSON.stringify(p.name)})`);
  }
  if (replaceInfo.hasValidate) {
    lines.push(`\t\tif err := req.Validate(); err != nil {`);
    lines.push(`\t\t\trt.WriteProblem(w, r, err)`);
    lines.push(`\t\t\treturn`);
    lines.push(`\t\t}`);
  }
  lines.push(`\t\tres, err := impl.${replaceInfo.methodName}(ctx, &req)`);
  lines.push(`\t\tif err != nil {`);
  lines.push(`\t\t\trt.WriteProblem(w, r, err)`);
  lines.push(`\t\t\treturn`);
  lines.push(`\t\t}`);
  lines.push(`\t\trt.WriteJSON(w, http.StatusOK, res)`);
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
