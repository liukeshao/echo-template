import { createTypeSpecLibrary } from "@typespec/compiler";

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
  diagnostics: {},
  emitter: {
    options: EmitterOptionsSchema,
  },
  state: {
    autoPatch: { description: "Auto-patch metadata linking PATCH to GET and REPLACE operations" },
  },
});

export const StateKeys = $lib.stateKeys;

export function $autoPatch(context, target, getOp, replaceOp) {
  context.program.stateMap(StateKeys.autoPatch).set(target, { getOp, replaceOp });
}

export function getAutoPatch(program, op) {
  return program.stateMap(StateKeys.autoPatch).get(op);
}
