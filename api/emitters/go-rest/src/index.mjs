import { emitFile, resolvePath } from "@typespec/compiler";
import { getAllHttpServices } from "@typespec/http";

import { $autoPatch, $lib, getAutoPatch } from "./lib.mjs";
import { emitModelStruct, emitParamRequestStruct, emitValidateMethod } from "./go-types.mjs";
import { emitInterfaceAndMount, operationMethodName, requiresAuth, resolveSuccessResponse } from "./go-server.mjs";

export { $lib };

export const $decorators = {
  GoRest: {
    autoPatch: $autoPatch,
  },
};

export async function $onEmit(context) {
  const { program, emitterOutputDir, options } = context;
  const pkgName = options["package-name"] || "api";
  const runtimeImport = options["runtime-import"] || "github.com/liukeshao/echo-template/pkg/httpruntime";
  const validatorImport = options["validator-import"] || "github.com/liukeshao/echo-template/pkg/validator";
  const utilsImport = options["utils-import"] || "github.com/liukeshao/echo-template/pkg/utils";

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
      utilsImport,
    });
    const fileName = `${ifaceName.toLowerCase()}.gen.go`;
    await emitFile(program, {
      path: resolvePath(emitterOutputDir, fileName),
      content: code,
    });
  }
}

function renderInterfaceFile(program, ifaceName, httpOps, cfg) {
  const imports = new Set(["context", "net/http", "httpruntime"]);
  const emittedModels = new Set();
  const typeBlocks = [];
  const opInfos = [];

  for (const httpOp of httpOps) {
    const autoPatch = getAutoPatch(program, httpOp.operation);
    const methodName = operationMethodName(ifaceName, httpOp.operation.name);
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
        emitReferencedModels(program, respType, emittedModels, typeBlocks, imports);
      }
    }

    let reqStruct = null;
    let hasValidate = false;

    if (!autoPatch) {
      if (bodyModel) {
        reqStruct = bodyModel.name;
        if (!emittedModels.has(bodyModel.name)) {
          emittedModels.add(bodyModel.name);
          const validateProps = [...pathParams.map((p) => p.param), ...bodyModel.properties.values()];
          hasValidate = Boolean(emitValidateMethod(program, bodyModel.name, validateProps, new Set()));
          typeBlocks.push(
            emitModelStruct(program, bodyModel, {
              extraPathFields: pathParams,
              emitValidate: true,
              imports,
            }),
          );
        }
      } else if (pathParams.length > 0 || queryParams.length > 0) {
        reqStruct = `${methodName}Request`;
        const allParams = [...pathParams, ...queryParams];
        hasValidate = Boolean(emitValidateMethod(program, reqStruct, allParams.map((p) => p.param), new Set()));
        if (queryParams.some((p) => p.param.type.kind === "Scalar" && (p.param.type.name === "int32" || p.param.type.name === "int64"))) {
          imports.add("validator");
        }
        typeBlocks.push(
          emitParamRequestStruct(program, reqStruct, allParams, {
            emitValidate: true,
            imports,
          }),
        );
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
      needAuth: requiresAuth(httpOp),
    });
  }

  const serverBlock = emitInterfaceAndMount(program, ifaceName, opInfos);

  const stdImports = [];
  const extImports = [];
  for (const imp of imports) {
    if (imp === "httpruntime") {
      extImports.push(cfg.runtimeImport);
    } else if (imp === "validator") {
      extImports.push(cfg.validatorImport);
    } else if (imp === "utils") {
      extImports.push(cfg.utilsImport);
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

function emitReferencedModels(program, model, emitted, blocks, imports) {
  if (!model || model.kind !== "Model" || !model.name || emitted.has(model.name)) return;
  if (model.name === "Array" || model.name === "Record") return;
  emitted.add(model.name);

  for (const prop of model.properties.values()) {
    if (prop.type.kind === "Model") {
      if (prop.type.name === "Array" && prop.type.indexer) {
        emitReferencedModels(program, prop.type.indexer.value, emitted, blocks, imports);
      } else if (prop.type.name !== "Record") {
        emitReferencedModels(program, prop.type, emitted, blocks, imports);
      }
    }
  }

  blocks.push(emitModelStruct(program, model, { emitValidate: false, imports }));
}
