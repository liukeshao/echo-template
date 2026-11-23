import "@typespec/http";
import "tsp-emitter-go-rest";

namespace EchoTemplate;

model JsonPatchOperation {
  `op`: "add" | "remove" | "replace" | "move" | "copy" | "test";
  path: string;
  from?: string;
  value?: unknown;
}
