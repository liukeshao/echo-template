import { emitFile, resolvePath } from "@typespec/compiler";
import { getAllHttpServices } from "@typespec/http";

import { $autoPatch, $lib, getAutoPatch } from "./lib.mjs";
import { baseScalarName, emitEnumType, emitModelStruct, emitParamRequestStruct } from "./go-types.mjs";
import { emitInterfaceAndMount, operationMethodName, resolveAuthOptions, resolveSuccessResponse } from "./go-server.mjs";

export { $lib };

export const $decorators = {
  GoRest: {
    autoPatch: $autoPatch,
  },
};

export async function $onEmit(context) {
  const { program, emitterOutputDir, options } = context;
  const pkgName = options["package-name"] || "api";
  const runtimeImport = options["runtime-import"] || "github.com/liukeshao/echo-template/pkg/rest";
  const validatorImport = options["validator-import"] || "github.com/liukeshao/echo-template/pkg/validator";

  const [services] = getAllHttpServices(program);
  if (!services || services.length === 0) return;

  // Group HTTP operations by TypeSpec interface name (e.g. Auth -> auth.gen.go, Me -> me.gen.go, Books -> books.gen.go).
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

  for (const [ifaceName, httpOps] of byInterface.entries()) {
    const code = renderInterfaceFile(program, ifaceName, httpOps, {
      pkgName,
      runtimeImport,
      validatorImport,
    });
    const fileName = `${ifaceName.toLowerCase()}.gen.go`;
    await emitFile(program, {
      path: resolvePath(emitterOutputDir, fileName),
      content: code,
    });
  }
}

function renderInterfaceFile(program, ifaceName, httpOps, cfg) {
  const imports = new Set(["context", "net/http", "rest"]);
  const emittedTypes = new Set();
  const validatedModels = new Set();
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
    const body = httpOp.parameters.body;
    const bodyModel = body && body.type && body.type.kind === "Model" && body.type.name && body.type.name !== "Record"
      ? body.type
      : null;

    const { status, modelName: respModel } = resolveSuccessResponse(httpOp);

    if (respModel) {
      const respType = findSuccessModel(httpOp);
      if (respType) {
        emitReferencedTypes(program, respType, emittedTypes, typeBlocks, imports);
      }
    }

    let reqStruct = null;
    let hasValidate = false;

    if (!autoPatch) {
      if (bodyModel) {
        reqStruct = bodyModel.name;
        if (!emittedTypes.has(bodyModel.name)) {
          emitReferencedPropertyTypes(program, bodyModel, emittedTypes, typeBlocks, imports);
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
          if (p.param.type.kind === "Enum") {
            emitReferencedTypes(program, p.param.type, emittedTypes, typeBlocks, imports);
          }
        }
        if (
          queryParams.some((p) => {
            const base = p.param.type.kind === "Scalar" ? baseScalarName(p.param.type) : "";
            return base === "int32" || base === "int64" || base === "float64" || base === "boolean";
          })
        ) {
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

  const stdImports = [];
  const extImports = [];
  for (const imp of imports) {
    if (imp === "rest") {
      extImports.push(cfg.runtimeImport);
    } else if (imp === "validator") {
      extImports.push(cfg.validatorImport);
    } else {
      stdImports.push(imp);
    }
  }
  stdImports.sort();
  extImports.sort();

  const importLines = ["import ("];
  for (const s of stdImports) importLines.push(`\t${JSON.stringify(s)}`);
  if (stdImports.length > 0 && extImports.length > 0) importLines.push("");
  for (const e of extImports) importLines.push(`\t${JSON.stringify(e)}`);
  importLines.push(")");

  return [
    "// Code generated by tsp-emitter-go-rest from TypeSpec. DO NOT EDIT.",
    `package ${cfg.pkgName}`,
    "",
    ...importLines,
    "",
    ...(extraDecls.length > 0 ? [...extraDecls, ""] : []),
    ...typeBlocks.flatMap((b) => [b, ""]),
    serverBlock,
    "",
  ].join("\n");
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

function emitReferencedPropertyTypes(program, model, emitted, blocks, imports) {
  for (const prop of model.properties.values()) {
    if (prop.type.kind === "Enum") {
      emitReferencedTypes(program, prop.type, emitted, blocks, imports);
    } else if (prop.type.kind === "Model") {
      if ((prop.type.name === "Array" || prop.type.name === "Record") && prop.type.indexer) {
        emitReferencedTypes(program, prop.type.indexer.value, emitted, blocks, imports);
      } else if (prop.type.name !== "Record") {
        emitReferencedTypes(program, prop.type, emitted, blocks, imports);
      }
    }
  }
}

function emitReferencedTypes(program, type, emitted, blocks, imports) {
  if (!type || !type.name || emitted.has(type.name)) return;
  if (type.kind === "Enum") {
    emitted.add(type.name);
    blocks.push(emitEnumType(program, type));
    return;
  }
  if (type.kind !== "Model") return;
  if (type.name === "Array" || type.name === "Record") return;
  emitted.add(type.name);

  emitReferencedPropertyTypes(program, type, emitted, blocks, imports);
  blocks.push(emitModelStruct(program, type, { emitValidate: false, imports }));
}
