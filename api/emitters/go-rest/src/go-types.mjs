import {
  getDoc,
  getMaxLength,
  getMinLength,
  getMinValue,
} from "@typespec/compiler";

const INITIALISMS = new Set(["id", "ulid", "url", "uri", "http", "ip", "jwt", "api", "json"]);

/** Convert snake_case / kebab-case / camelCase to Go PascalCase with idiomatic initialisms. */
export function toGoName(name) {
  const words = name
    .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
    .split(/[^a-zA-Z0-9]+/)
    .filter(Boolean);
  return words
    .map((w) => {
      const lower = w.toLowerCase();
      if (INITIALISMS.has(lower)) return lower.toUpperCase();
      return lower.charAt(0).toUpperCase() + lower.slice(1);
    })
    .join("");
}

/** Return the semantic scalar name if `type` is (or derives from) one of our custom scalars. */
export function semanticScalarName(type) {
  const semantic = new Set(["ULID", "Email", "Username", "Password"]);
  let cur = type;
  while (cur && cur.kind === "Scalar") {
    if (semantic.has(cur.name)) return cur.name;
    cur = cur.baseScalar;
  }
  return null;
}

/** Map a TypeSpec type + optionality to Go type string and required imports. */
export function resolveGoType(type, optional, imports) {
  if (type.kind === "Scalar") {
    const base = baseScalarName(type);
    switch (base) {
      case "string":
        return "string";
      case "int32":
        return "int32";
      case "int64":
        return "int64";
      case "float64":
        return optional ? "*float64" : "float64";
      case "boolean":
        return optional ? "*bool" : "bool";
      case "utcDateTime":
      case "offsetDateTime":
        imports.add("time");
        return optional ? "*time.Time" : "time.Time";
      default:
        return "string";
    }
  }
  if (type.kind === "Model") {
    if (type.name === "Array" && type.indexer) {
      const elem = resolveGoType(type.indexer.value, false, imports);
      return `[]${elem}`;
    }
    if (type.name === "Record" && type.indexer) {
      const val = resolveGoType(type.indexer.value, false, imports);
      return `map[string]${val}`;
    }
    return optional ? `*${type.name}` : type.name;
  }
  return "any";
}

function baseScalarName(scalar) {
  const builtin = new Set(["string", "int32", "int64", "float64", "boolean", "utcDateTime", "offsetDateTime"]);
  let cur = scalar;
  while (cur && cur.kind === "Scalar") {
    if (builtin.has(cur.name)) return cur.name;
    cur = cur.baseScalar;
  }
  return scalar.name;
}

/**
 * Emit a flat Go struct for a TypeSpec model, optionally prepending extra path fields
 * (e.g. `Book string \`json:"-"\`` on ReplaceBookRequest).
 */
export function emitModelStruct(program, model, { extraPathFields = [], emitValidate = false, imports }) {
  const lines = [];
  const doc = getDoc(program, model);
  if (doc) {
    lines.push(`// ${model.name} ${doc}`);
  }
  lines.push(`type ${model.name} struct {`);

  for (const pf of extraPathFields) {
    lines.push(`\t${toGoName(pf.name)} string \`json:"-"\``);
  }

  for (const prop of model.properties.values()) {
    const goField = toGoName(prop.name);
    const goType = resolveGoType(prop.type, prop.optional, imports);
    const omitempty = prop.optional ? ",omitempty" : "";
    const propDoc = getDoc(program, prop);
    if (propDoc) {
      lines.push(`\t// ${goField} ${propDoc}`);
    }
    lines.push(`\t${goField} ${goType} \`json:"${prop.name}${omitempty}"\``);
  }
  lines.push("}");

  if (emitValidate) {
    const validateProps = [
      ...extraPathFields.map((pf) => pf.param),
      ...model.properties.values(),
    ];
    const vCode = emitValidateMethod(program, model.name, validateProps, imports);
    if (vCode) {
      lines.push("");
      lines.push(vCode);
    }
  }
  return lines.join("\n");
}

/** Emit a request struct constructed from HTTP parameters (path/query) when there is no body model. */
export function emitParamRequestStruct(program, structName, params, { emitValidate = false, imports }) {
  const lines = [`type ${structName} struct {`];
  const props = [];
  for (const p of params) {
    const prop = p.param;
    props.push(prop);
    const goField = toGoName(prop.name);
    const goType = resolveGoType(prop.type, false, imports);
    lines.push(`\t${goField} ${goType}`);
  }
  lines.push("}");

  if (emitValidate) {
    const vCode = emitValidateMethod(program, structName, props, imports);
    if (vCode) {
      lines.push("");
      lines.push(vCode);
    }
  }
  return lines.join("\n");
}

/** Generate Validate() error directly targeting pkg/validator (with i18n keys) and optional validateExtra hook. */
export function emitValidateMethod(program, structName, props, imports) {
  const checks = [];

  for (const prop of props) {
    const goField = toGoName(prop.name);
    const jsonName = prop.name;
    const semantic = semanticScalarName(prop.type);

    if (semantic === "ULID") {
      imports.add("utils");
      if (prop.optional) {
        checks.push(
          `\tif r.${goField} != "" && !utils.IsValidULID(r.${goField}) {`,
          `\t\tv.Add(${JSON.stringify(jsonName)}, validator.KeyULID, map[string]string{"field": ${JSON.stringify(jsonName)}})`,
          `\t}`,
        );
      } else {
        checks.push(
          `\tif !utils.IsValidULID(r.${goField}) {`,
          `\t\tv.Add(${JSON.stringify(jsonName)}, validator.KeyULID, map[string]string{"field": ${JSON.stringify(jsonName)}})`,
          `\t}`,
        );
      }
      continue;
    }

    if (semantic === "Email") {
      checks.push(
        `\tif !validator.IsValidEmail(r.${goField}) {`,
        `\t\tv.Add(${JSON.stringify(jsonName)}, validator.KeyEmail, map[string]string{"field": ${JSON.stringify(jsonName)}})`,
        `\t}`,
      );
      continue;
    }

    if (semantic === "Username") {
      checks.push(`\tvalidator.CheckUsername(&v, ${JSON.stringify(jsonName)}, r.${goField})`);
      continue;
    }

    if (semantic === "Password") {
      checks.push(`\tvalidator.CheckPassword(&v, ${JSON.stringify(jsonName)}, r.${goField})`);
      continue;
    }

    const minLen = getMinLength(program, prop) ?? (prop.type.kind === "Scalar" ? getMinLength(program, prop.type) : undefined);
    const maxLen = getMaxLength(program, prop) ?? (prop.type.kind === "Scalar" ? getMaxLength(program, prop.type) : undefined);
    const minVal = getMinValue(program, prop);
    const base = prop.type.kind === "Scalar" ? baseScalarName(prop.type) : "";

    if (base === "string") {
      if (!prop.optional && minLen && minLen >= 1) {
        imports.add("strings");
        if (maxLen !== undefined) {
          checks.push(
            `\tif strings.TrimSpace(r.${goField}) == "" {`,
            `\t\tv.Add(${JSON.stringify(jsonName)}, validator.KeyRequired, map[string]string{"field": ${JSON.stringify(jsonName)}})`,
            `\t} else if len(r.${goField}) > ${maxLen} {`,
            `\t\tv.Add(${JSON.stringify(jsonName)}, validator.KeyMaxLength, map[string]string{"field": ${JSON.stringify(jsonName)}, "max": ${JSON.stringify(String(maxLen))}})`,
            `\t}`,
          );
        } else {
          checks.push(
            `\tif strings.TrimSpace(r.${goField}) == "" {`,
            `\t\tv.Add(${JSON.stringify(jsonName)}, validator.KeyRequired, map[string]string{"field": ${JSON.stringify(jsonName)}})`,
            `\t}`,
          );
        }
      } else if (maxLen !== undefined) {
        checks.push(
          `\tif len(r.${goField}) > ${maxLen} {`,
          `\t\tv.Add(${JSON.stringify(jsonName)}, validator.KeyMaxLength, map[string]string{"field": ${JSON.stringify(jsonName)}, "max": ${JSON.stringify(String(maxLen))}})`,
          `\t}`,
        );
      }
    }

    if (base === "float64" && minVal === 0) {
      if (prop.optional) {
        checks.push(
          `\tif r.${goField} != nil && *r.${goField} < 0 {`,
          `\t\tv.Add(${JSON.stringify(jsonName)}, validator.KeyNonNegative, map[string]string{"field": ${JSON.stringify(jsonName)}})`,
          `\t}`,
        );
      } else {
        checks.push(
          `\tif r.${goField} < 0 {`,
          `\t\tv.Add(${JSON.stringify(jsonName)}, validator.KeyNonNegative, map[string]string{"field": ${JSON.stringify(jsonName)}})`,
          `\t}`,
        );
      }
    }

    if ((base === "int32" || base === "int64") && minVal === 0) {
      checks.push(
        `\tif r.${goField} < 0 {`,
        `\t\tv.Add(${JSON.stringify(jsonName)}, validator.KeyNonNegative, map[string]string{"field": ${JSON.stringify(jsonName)}})`,
        `\t}`,
      );
    }
  }

  if (checks.length === 0) {
    return null;
  }

  imports.add("validator");
  return [
    `// Validate 根据 TypeSpec 约束自动生成字段级校验，返回本地化 *validator.FieldViolations。`,
    `// 若同包手写文件实现了 validateExtra(*validator.FieldViolations)，会自动追加跨字段校验。`,
    `func (r *${structName}) Validate() error {`,
    `\tvar v validator.FieldViolations`,
    ...checks,
    `\tif extra, ok := any(r).(interface{ validateExtra(*validator.FieldViolations) }); ok {`,
    `\t\textra.validateExtra(&v)`,
    `\t}`,
    `\treturn v.Err()`,
    `}`,
  ].join("\n");
}
