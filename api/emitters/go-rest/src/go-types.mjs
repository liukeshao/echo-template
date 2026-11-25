import {
  getDoc,
  getMaxLength,
  getMaxValue,
  getMinLength,
  getMinValue,
  getPattern,
} from "@typespec/compiler";

const INITIALISMS = new Set(["id", "uuid", "url", "uri", "http", "ip", "jwt", "api", "json"]);

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
  const semantic = new Set(["UUID", "Email", "Username", "Password"]);
  let cur = type;
  while (cur && cur.kind === "Scalar") {
    if (semantic.has(cur.name)) return cur.name;
    cur = cur.baseScalar;
  }
  return null;
}

/** Return allowed string literal values if `type` is a string Enum or a Union of string literals. */
export function stringLiteralValues(type) {
  if (!type) return null;
  if (type.kind === "Enum") {
    const vals = [];
    for (const member of type.members.values()) {
      const v = member.value ?? member.name;
      if (typeof v !== "string") return null;
      vals.push(v);
    }
    return vals.length > 0 ? vals : null;
  }
  if (type.kind === "Union") {
    const vals = [];
    for (const variant of type.variants.values()) {
      if (variant.type.kind !== "String") return null;
      vals.push(variant.type.value);
    }
    return vals.length > 0 ? vals : null;
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
  if (type.kind === "Enum") {
    return type.name;
  }
  if (type.kind === "Union" && stringLiteralValues(type)) {
    return "string";
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

export function baseScalarName(scalar) {
  const builtin = new Set(["string", "int32", "int64", "float64", "boolean", "utcDateTime", "offsetDateTime"]);
  let cur = scalar;
  while (cur && cur.kind === "Scalar") {
    if (builtin.has(cur.name)) return cur.name;
    cur = cur.baseScalar;
  }
  return scalar.name;
}

/** Emit a Go typed string enum and its constants from a TypeSpec Enum. */
export function emitEnumType(program, enumType) {
  const lines = [];
  const doc = getDoc(program, enumType);
  if (doc) {
    lines.push(`// ${enumType.name} ${doc}`);
  }
  lines.push(`type ${enumType.name} string`);
  lines.push("");
  lines.push("const (");
  for (const member of enumType.members.values()) {
    const val = member.value ?? member.name;
    const constName = `${enumType.name}${toGoName(member.name)}`;
    const memberDoc = getDoc(program, member);
    if (memberDoc) {
      lines.push(`\t// ${constName} ${memberDoc}`);
    }
    lines.push(`\t${constName} ${enumType.name} = ${JSON.stringify(String(val))}`);
  }
  lines.push(")");
  return lines.join("\n");
}

/**
 * Emit a flat Go struct for a TypeSpec model, optionally prepending extra path fields
 * (e.g. `Book string \`json:"-"\`` on ReplaceBookRequest).
 */
export function emitModelStruct(
  program,
  model,
  { extraPathFields = [], emitValidate = false, imports, validatedModels, extraDecls },
) {
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
    const vCode = emitValidateMethod(program, model.name, validateProps, imports, extraDecls);
    if (vCode) {
      if (validatedModels) validatedModels.add(model.name);
      lines.push("");
      lines.push(vCode);
    }
  }
  return lines.join("\n");
}

/** Emit a request struct constructed from HTTP parameters (path/query) when there is no body model. */
export function emitParamRequestStruct(
  program,
  structName,
  params,
  { emitValidate = false, imports, validatedModels, extraDecls },
) {
  const lines = [`type ${structName} struct {`];
  const props = [];
  for (const p of params) {
    const prop = p.param;
    props.push(prop);
    const goField = toGoName(prop.name);
    const goType = resolveGoType(prop.type, prop.optional, imports);
    lines.push(`\t${goField} ${goType}`);
  }
  lines.push("}");

  if (emitValidate) {
    const vCode = emitValidateMethod(program, structName, props, imports, extraDecls);
    if (vCode) {
      if (validatedModels) validatedModels.add(structName);
      lines.push("");
      lines.push(vCode);
    }
  }
  return lines.join("\n");
}

/**
 * Generate Validate() error directly targeting pkg/validator (with i18n keys) and optional validateExtra hook.
 * Always emitted for request structs so hand-written validateExtra(*validator.FieldViolations) hooks work
 * even when a request struct has no single-field TypeSpec decorators.
 */
export function emitValidateMethod(program, structName, props, imports, extraDecls = []) {
  const checks = [];

  for (const prop of props) {
    const goField = toGoName(prop.name);
    const jsonName = prop.name;
    const semantic = semanticScalarName(prop.type);

    if (semantic === "UUID") {
      if (prop.optional) {
        checks.push(
          `\tif r.${goField} != "" && !validator.IsValidUUID(r.${goField}) {`,
          `\t\tv.Add(${JSON.stringify(jsonName)}, validator.CodeUUID, nil)`,
          `\t}`,
        );
      } else {
        checks.push(
          `\tif !validator.IsValidUUID(r.${goField}) {`,
          `\t\tv.Add(${JSON.stringify(jsonName)}, validator.CodeUUID, nil)`,
          `\t}`,
        );
      }
      continue;
    }

    if (semantic === "Email") {
      if (prop.optional) {
        checks.push(
          `\tif r.${goField} != "" && !validator.IsValidEmail(r.${goField}) {`,
          `\t\tv.Add(${JSON.stringify(jsonName)}, validator.CodeEmail, nil)`,
          `\t}`,
        );
      } else {
        checks.push(
          `\tif !validator.IsValidEmail(r.${goField}) {`,
          `\t\tv.Add(${JSON.stringify(jsonName)}, validator.CodeEmail, nil)`,
          `\t}`,
        );
      }
      continue;
    }

    if (semantic === "Username") {
      if (prop.optional) {
        checks.push(
          `\tif r.${goField} != "" {`,
          `\t\tvalidator.CheckUsername(&v, ${JSON.stringify(jsonName)}, r.${goField})`,
          `\t}`,
        );
      } else {
        checks.push(`\tvalidator.CheckUsername(&v, ${JSON.stringify(jsonName)}, r.${goField})`);
      }
      continue;
    }

    if (semantic === "Password") {
      if (prop.optional) {
        checks.push(
          `\tif r.${goField} != "" {`,
          `\t\tvalidator.CheckPassword(&v, ${JSON.stringify(jsonName)}, r.${goField})`,
          `\t}`,
        );
      } else {
        checks.push(`\tvalidator.CheckPassword(&v, ${JSON.stringify(jsonName)}, r.${goField})`);
      }
      continue;
    }

    const enumVals = stringLiteralValues(prop.type);
    if (enumVals) {
      const allowedStr = enumVals.join(", ");
      const cases = enumVals.map((v) => JSON.stringify(v)).join(", ");
      const expr = prop.type.kind === "Enum" ? `string(r.${goField})` : `r.${goField}`;
      if (prop.optional) {
        checks.push(
          `\tif ${expr} != "" {`,
          `\t\tswitch ${expr} {`,
          `\t\tcase ${cases}:`,
          `\t\tdefault:`,
          `\t\t\tv.Add(${JSON.stringify(jsonName)}, validator.CodeOneOf, map[string]string{"allowed": ${JSON.stringify(allowedStr)}})`,
          `\t\t}`,
          `\t}`,
        );
      } else {
        checks.push(
          `\tswitch ${expr} {`,
          `\tcase ${cases}:`,
          `\tdefault:`,
          `\t\tv.Add(${JSON.stringify(jsonName)}, validator.CodeOneOf, map[string]string{"allowed": ${JSON.stringify(allowedStr)}})`,
          `\t}`,
        );
      }
      continue;
    }

    const minLen = getMinLength(program, prop) ?? (prop.type.kind === "Scalar" ? getMinLength(program, prop.type) : undefined);
    const maxLen = getMaxLength(program, prop) ?? (prop.type.kind === "Scalar" ? getMaxLength(program, prop.type) : undefined);
    const pattern = getPattern(program, prop) ?? (prop.type.kind === "Scalar" ? getPattern(program, prop.type) : undefined);
    const minVal = getMinValue(program, prop) ?? (prop.type.kind === "Scalar" ? getMinValue(program, prop.type) : undefined);
    const maxVal = getMaxValue(program, prop) ?? (prop.type.kind === "Scalar" ? getMaxValue(program, prop.type) : undefined);
    const base = prop.type.kind === "Scalar" ? baseScalarName(prop.type) : "";

    if (base === "string") {
      emitStringChecks(checks, { structName, goField, jsonName, optional: prop.optional, minLen, maxLen, pattern, imports, extraDecls });
    }

    if (base === "float64" || base === "int32" || base === "int64") {
      const isPtr = base === "float64" && prop.optional;
      emitNumericChecks(checks, { goField, jsonName, isPtr, minVal, maxVal });
    }
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

function emitStringChecks(checks, { structName, goField, jsonName, optional, minLen, maxLen, pattern, imports, extraDecls }) {
  const branches = [];

  if (!optional && minLen !== undefined && minLen >= 1) {
    imports.add("strings");
    branches.push({
      cond: `strings.TrimSpace(r.${goField}) == ""`,
      body: `v.Add(${JSON.stringify(jsonName)}, validator.CodeRequired, nil)`,
    });
    if (minLen > 1) {
      imports.add("unicode/utf8");
      branches.push({
        cond: `utf8.RuneCountInString(r.${goField}) < ${minLen}`,
        body: `v.Add(${JSON.stringify(jsonName)}, validator.CodeMinLength, map[string]string{"min": ${JSON.stringify(String(minLen))}})`,
      });
    }
  } else if (optional && minLen !== undefined && minLen >= 1) {
    imports.add("unicode/utf8");
    branches.push({
      cond: `r.${goField} != "" && utf8.RuneCountInString(r.${goField}) < ${minLen}`,
      body: `v.Add(${JSON.stringify(jsonName)}, validator.CodeMinLength, map[string]string{"min": ${JSON.stringify(String(minLen))}})`,
    });
  }

  if (maxLen !== undefined) {
    imports.add("unicode/utf8");
    branches.push({
      cond: `utf8.RuneCountInString(r.${goField}) > ${maxLen}`,
      body: `v.Add(${JSON.stringify(jsonName)}, validator.CodeMaxLength, map[string]string{"max": ${JSON.stringify(String(maxLen))}})`,
    });
  }

  if (pattern !== undefined) {
    imports.add("regexp");
    const varName = `pattern${structName}${goField}`;
    extraDecls.push(`var ${varName} = regexp.MustCompile(${JSON.stringify(pattern)})`);
    const guard = optional ? `r.${goField} != "" && ` : "";
    branches.push({
      cond: `${guard}!${varName}.MatchString(r.${goField})`,
      body: `v.Add(${JSON.stringify(jsonName)}, validator.CodePattern, map[string]string{"pattern": ${JSON.stringify(pattern)}})`,
    });
  }

  if (branches.length === 0) return;

  branches.forEach((b, idx) => {
    const prefix = idx === 0 ? "\tif " : "\t} else if ";
    checks.push(`${prefix}${b.cond} {`);
    checks.push(`\t\t${b.body}`);
  });
  checks.push("\t}");
}

function emitNumericChecks(checks, { goField, jsonName, isPtr, minVal, maxVal }) {
  const valExpr = isPtr ? `*r.${goField}` : `r.${goField}`;
  const guard = isPtr ? `r.${goField} != nil && ` : "";

  if (minVal !== undefined) {
    if (minVal === 0) {
      checks.push(
        `\tif ${guard}${valExpr} < 0 {`,
        `\t\tv.Add(${JSON.stringify(jsonName)}, validator.CodeNonNegative, map[string]string{"min": "0"})`,
        `\t}`,
      );
    } else {
      checks.push(
        `\tif ${guard}${valExpr} < ${minVal} {`,
        `\t\tv.Add(${JSON.stringify(jsonName)}, validator.CodeMinValue, map[string]string{"min": ${JSON.stringify(String(minVal))}})`,
        `\t}`,
      );
    }
  }

  if (maxVal !== undefined) {
    checks.push(
      `\tif ${guard}${valExpr} > ${maxVal} {`,
      `\t\tv.Add(${JSON.stringify(jsonName)}, validator.CodeMaxValue, map[string]string{"max": ${JSON.stringify(String(maxVal))}})`,
      `\t}`,
    );
  }
}
