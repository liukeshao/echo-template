import { emitFile, resolvePath } from "@typespec/compiler";
import { getAllHttpServices } from "@typespec/http";

import { $autoPatch, $lib, getAutoPatch } from "./lib.mjs";
import { emitEnumType, emitModelStruct, emitParamRequestStruct } from "./go-types.mjs";
import {
  emitInterfaceAndMount,
  needsTypedQueryValidator,
  operationMethodName,
  resolveAuthOptions,
  resolveSuccessResponse,
} from "./go-server.mjs";

export { $lib };

export const $decorators = {
  GoRest: {
    autoPatch: $autoPatch,
  },
};

export async function $onEmit(context) {
  const { program, emitterOutputDir, options } = context;
  const pkgName = options["package-name"] || "api";
  const runtimeImport = options["runtime-import"] || "github.com/liukeshao/go-rest-template/pkg/httpkit";
  const validatorImport = options["validator-import"] || "github.com/liukeshao/go-rest-template/pkg/validator";

  const [services] = getAllHttpServices(program);
  if (!services || services.length === 0) return;

  const byInterface = new Map();
  for (const svc of services) {
    for (const httpOp of svc.operations) {
      const iface = httpOp.operation.interface;
      const ifaceName = iface ? iface.name : svc.namespace.name;
      if (!byInterface.has(ifaceName)) {
        byInterface.set(ifaceName, []);
      }
      byInterface.get(ifaceName).push(httpOp);
    }
  }

  // Pass 1: 跨接口扫描所有被引用的 Model / Enum 以及需要 Validate() 的嵌套请求模型。
  // 被多个接口共同引用的类型提取到 types.gen.go，避免不同 *.gen.go 出现重复声明导致编译失败。
  const typeMeta = new Map(); // name -> { type, interfaces: Set<string>, needsValidate: boolean }
  for (const [ifaceName, httpOps] of byInterface.entries()) {
    for (const httpOp of httpOps) {
      const autoPatch = getAutoPatch(program, httpOp.operation);
      const respType = findSuccessModel(httpOp);
      if (respType) {
        collectReferencedTypes(respType, ifaceName, false, typeMeta, new Set());
      }
      if (!autoPatch) {
        const pathParams = httpOp.parameters.parameters.filter((p) => p.type === "path");
        const queryParams = httpOp.parameters.parameters.filter((p) => p.type === "query");
        const bodyModel = findBodyModel(httpOp);
        if (bodyModel) {
          // 带额外 path 字段的顶层请求模型归属具体接口，但嵌套子类型可跨接口共享。
          if (pathParams.length === 0) {
            collectReferencedTypes(bodyModel, ifaceName, true, typeMeta, new Set());
          } else {
            collectPropertyTypes(bodyModel, ifaceName, true, typeMeta, new Set());
          }
        }
        for (const p of [...pathParams, ...queryParams]) {
          collectReferencedTypes(p.param.type, ifaceName, false, typeMeta, new Set());
        }
      }
    }
  }

  const sharedEmitted = new Set();
  const globalValidatedModels = new Set();
  const sharedTypeEntries = [...typeMeta.entries()].filter(([, meta]) => meta.interfaces.size > 1);

  if (sharedTypeEntries.length > 0) {
    const sharedCode = renderSharedTypesFile(program, sharedTypeEntries, sharedEmitted, globalValidatedModels, {
      pkgName,
      runtimeImport,
      validatorImport,
    });
    if (sharedCode) {
      await emitFile(program, {
        path: resolvePath(emitterOutputDir, "types.gen.go"),
        content: sharedCode,
      });
    }
  }

  // Pass 2: 逐接口生成 DTO、Validate()、Handler 接口与 NewXxxHandler 适配器。
  for (const [ifaceName, httpOps] of byInterface.entries()) {
    const code = renderInterfaceFile(program, ifaceName, httpOps, {
      pkgName,
      runtimeImport,
      validatorImport,
      sharedEmitted,
      globalValidatedModels,
      typeMeta,
    });
    const fileName = `${ifaceName.toLowerCase()}.gen.go`;
    await emitFile(program, {
      path: resolvePath(emitterOutputDir, fileName),
      content: code,
    });
  }
}

function renderSharedTypesFile(program, sharedEntries, sharedEmitted, globalValidatedModels, cfg) {
  const imports = new Set();
  const typeBlocks = [];
  const extraDecls = [];
  const sharedNames = new Set(sharedEntries.map(([name]) => name));

  const emitShared = (type, needsValidate) => {
    if (!type || !type.name || sharedEmitted.has(type.name)) return;
    if (!sharedNames.has(type.name)) return;
    if (type.kind === "Enum") {
      sharedEmitted.add(type.name);
      typeBlocks.push(emitEnumType(program, type));
      return;
    }
    if (type.kind !== "Model" || type.name === "Array" || type.name === "Record") return;
    sharedEmitted.add(type.name);
    for (const prop of type.properties.values()) {
      for (const nested of unwrapNamedTypes(prop.type)) {
        emitShared(nested, needsValidate);
      }
    }
    typeBlocks.push(
      emitModelStruct(program, type, {
        emitValidate: needsValidate,
        imports,
        validatedModels: globalValidatedModels,
        extraDecls,
      }),
    );
  };

  for (const [, meta] of sharedEntries) {
    emitShared(meta.type, meta.needsValidate);
  }

  if (typeBlocks.length === 0) return "";
  return formatGoFile(cfg, imports, extraDecls, typeBlocks);
}

function renderInterfaceFile(program, ifaceName, httpOps, cfg) {
  const imports = new Set(["context", "net/http", "httpkit"]);
  const emittedTypes = new Set(cfg.sharedEmitted);
  const validatedModels = new Set(cfg.globalValidatedModels);
  const typeBlocks = [];
  const extraDecls = [];
  const opInfos = [];
  const seenAutoPatchRoutes = new Set();

  for (const httpOp of httpOps) {
    const autoPatch = getAutoPatch(program, httpOp.operation);
    if (autoPatch) {
      const routeKey = `${httpOp.verb.toUpperCase()} ${httpOp.path}`;
      if (seenAutoPatchRoutes.has(routeKey)) {
        continue;
      }
      seenAutoPatchRoutes.add(routeKey);
    }

    const methodName = operationMethodName(httpOp.operation.name);
    const pathParams = httpOp.parameters.parameters.filter((p) => p.type === "path");
    const queryParams = httpOp.parameters.parameters.filter((p) => p.type === "query");
    const bodyModel = findBodyModel(httpOp);

    const { status, modelName: respModel } = resolveSuccessResponse(httpOp);

    if (respModel) {
      const respType = findSuccessModel(httpOp);
      if (respType) {
        emitReferencedTypes(program, respType, emittedTypes, typeBlocks, imports, {
          validatedModels,
          extraDecls,
          typeMeta: cfg.typeMeta,
          emitValidate: false,
        });
      }
    }

    let reqStruct = null;
    let hasValidate = false;

    if (!autoPatch) {
      if (bodyModel) {
        reqStruct = bodyModel.name;
        if (!emittedTypes.has(bodyModel.name)) {
          emitReferencedPropertyTypes(program, bodyModel, emittedTypes, typeBlocks, imports, {
            validatedModels,
            extraDecls,
            typeMeta: cfg.typeMeta,
            emitValidate: true,
          });
          emittedTypes.add(bodyModel.name);
          typeBlocks.push(
            emitModelStruct(program, bodyModel, {
              extraPathFields: pathParams,
              emitValidate: true,
              imports,
              validatedModels,
              extraDecls,
            }),
          );
        }
        hasValidate = validatedModels.has(bodyModel.name);
      } else if (pathParams.length > 0 || queryParams.length > 0) {
        reqStruct = `${methodName}Request`;
        const allParams = [...pathParams, ...queryParams];
        for (const p of allParams) {
          for (const named of unwrapNamedTypes(p.param.type)) {
            if (named.kind === "Enum") {
              emitReferencedTypes(program, named, emittedTypes, typeBlocks, imports, {
                validatedModels,
                extraDecls,
                typeMeta: cfg.typeMeta,
                emitValidate: false,
              });
            }
          }
        }
        if (needsTypedQueryValidator(queryParams)) {
          imports.add("validator");
        }
        typeBlocks.push(
          emitParamRequestStruct(program, reqStruct, allParams, {
            emitValidate: true,
            imports,
            validatedModels,
            extraDecls,
          }),
        );
        hasValidate = validatedModels.has(reqStruct);
      }
    }

    opInfos.push({
      httpOp,
      autoPatch,
      methodName,
      pathParams,
      queryParams,
      bodyModel,
      reqStruct,
      hasValidate,
      status,
      respModel,
      authOptions: resolveAuthOptions(httpOp),
    });
  }

  const serverBlock = emitInterfaceAndMount(program, ifaceName, opInfos);
  return formatGoFile(cfg, imports, extraDecls, [...typeBlocks, serverBlock]);
}

function formatGoFile(cfg, imports, extraDecls, blocks) {
  const stdImports = [];
  const extImports = [];
  for (const imp of imports) {
    if (imp === "httpkit") {
      extImports.push(cfg.runtimeImport);
    } else if (imp === "validator") {
      extImports.push(cfg.validatorImport);
    } else {
      stdImports.push(imp);
    }
  }
  stdImports.sort();
  extImports.sort();

  const importLines = [];
  if (stdImports.length > 0 || extImports.length > 0) {
    importLines.push("import (");
    for (const s of stdImports) importLines.push(`\t${JSON.stringify(s)}`);
    if (stdImports.length > 0 && extImports.length > 0) importLines.push("");
    for (const e of extImports) importLines.push(`\t${JSON.stringify(e)}`);
    importLines.push(")", "");
  }

  return [
    "// Code generated by tsp-emitter-go-rest from TypeSpec. DO NOT EDIT.",
    `package ${cfg.pkgName}`,
    "",
    ...importLines,
    ...(extraDecls.length > 0 ? [...extraDecls, ""] : []),
    ...blocks.flatMap((b) => [b, ""]),
  ].join("\n");
}

function findBodyModel(httpOp) {
  const body = httpOp.parameters.body;
  if (body && body.type && body.type.kind === "Model" && body.type.name && body.type.name !== "Record") {
    return body.type;
  }
  return null;
}

function findSuccessModel(httpOp) {
  for (const resp of httpOp.responses) {
    const code = typeof resp.statusCodes === "number" ? resp.statusCodes : 200;
    if (code >= 200 && code < 300) {
      for (const r of resp.responses) {
        if (r.body && r.body.type && r.body.type.kind === "Model" && r.body.type.name) {
          return r.body.type;
        }
      }
    }
  }
  return null;
}

function unwrapNamedTypes(type) {
  if (!type) return [];
  if (type.kind === "Enum") return [type];
  if (type.kind === "Model") {
    if ((type.name === "Array" || type.name === "Record") && type.indexer) {
      return unwrapNamedTypes(type.indexer.value);
    }
    if (type.name && type.name !== "Record") {
      return [type];
    }
  }
  return [];
}

function collectPropertyTypes(model, ifaceName, needsValidate, typeMeta, visiting) {
  for (const prop of model.properties.values()) {
    for (const named of unwrapNamedTypes(prop.type)) {
      collectReferencedTypes(named, ifaceName, needsValidate, typeMeta, visiting);
    }
  }
}

function collectReferencedTypes(type, ifaceName, needsValidate, typeMeta, visiting) {
  for (const named of unwrapNamedTypes(type)) {
    const entry = typeMeta.get(named.name) || {
      type: named,
      interfaces: new Set(),
      needsValidate: false,
    };
    entry.interfaces.add(ifaceName);
    if (needsValidate && named.kind === "Model") {
      entry.needsValidate = true;
    }
    typeMeta.set(named.name, entry);

    const visitKey = `${named.name}:${needsValidate ? 1 : 0}`;
    if (named.kind === "Model" && !visiting.has(visitKey)) {
      visiting.add(visitKey);
      collectPropertyTypes(named, ifaceName, needsValidate, typeMeta, visiting);
    }
  }
}

function emitReferencedPropertyTypes(program, model, emitted, blocks, imports, ctx) {
  for (const prop of model.properties.values()) {
    for (const named of unwrapNamedTypes(prop.type)) {
      emitReferencedTypes(program, named, emitted, blocks, imports, ctx);
    }
  }
}

function emitReferencedTypes(program, type, emitted, blocks, imports, ctx) {
  if (!type || !type.name || emitted.has(type.name)) return;
  if (type.kind === "Enum") {
    emitted.add(type.name);
    blocks.push(emitEnumType(program, type));
    return;
  }
  if (type.kind !== "Model" || type.name === "Array" || type.name === "Record") return;
  emitted.add(type.name);

  const meta = ctx.typeMeta ? ctx.typeMeta.get(type.name) : null;
  const shouldValidate = Boolean(ctx.emitValidate || (meta && meta.needsValidate));

  emitReferencedPropertyTypes(program, type, emitted, blocks, imports, {
    ...ctx,
    emitValidate: shouldValidate,
  });
  blocks.push(
    emitModelStruct(program, type, {
      emitValidate: shouldValidate,
      imports,
      validatedModels: ctx.validatedModels,
      extraDecls: ctx.extraDecls,
    }),
  );
}
