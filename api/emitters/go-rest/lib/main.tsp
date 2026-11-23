import "../src/index.mjs";

using TypeSpec.Reflection;

namespace GoRest;

/**
 * Marks a PATCH operation to be auto-implemented by the Go emitter as:
 *   getOp -> rt.ApplyPatch -> Validate -> replaceOp
 * Supports both parameterized resources (/books/{book}) and singletons (/me).
 */
extern dec autoPatch(target: Operation, getOp: Operation, replaceOp: Operation);
