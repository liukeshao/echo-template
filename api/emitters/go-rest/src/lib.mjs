import { createTypeSpecLibrary, paramMessage } from "@typespec/compiler";

const EmitterOptionsSchema = {
  type: "object",
  additionalProperties: false,
  properties: {
    "package-name": { type: "string", nullable: true },
    "runtime-import": { type: "string", nullable: true },
    "validator-import": { type: "string", nullable: true },
  },
  required: [],
};

export const $lib = createTypeSpecLibrary({
  name: "tsp-emitter-go-rest",
  diagnostics: {
    "invalid-auto-patch": {
      severity: "error",
      messages: {
        default: paramMessage`@autoPatch on ${"op"} must reference getOp and replaceOp defined within the same interface.`,
      },
    },
    "unsupported-query-type": {
      severity: "error",
      messages: {
        default: paramMessage`Query parameter "${"param"}" on operation "${"op"}" uses unsupported type "${"type"}".`,
      },
    },
  },
  emitter: {
    options: EmitterOptionsSchema,
  },
  state: {
    autoPatch: { description: "Auto-patch metadata linking PATCH to GET and REPLACE operations" },
  },
});

export const StateKeys = $lib.stateKeys;

export function reportDiagnostic(program, diagnostic) {
  $lib.reportDiagnostic(program, diagnostic);
}

export function $autoPatch(context, target, getOp, replaceOp) {
  context.program.stateMap(StateKeys.autoPatch).set(target, { getOp, replaceOp });
}

export function getAutoPatch(program, op) {
  return program.stateMap(StateKeys.autoPatch).get(op);
}
