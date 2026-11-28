import {
  getDoc,
  getMaxItems,
  getMaxLength,
  getMaxValue,
  getMinItems,
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

/** Return true when a TypeSpec property is optional without a default value (represented as a Go pointer for value types). */
export function isOptionalPointer(prop) {
  return Boolean(prop && prop.optional && prop.defaultValue === undefined);
}

/** Return the semantic scalar name if `type` is (or derives from) a scalar with special Go validation semantics. */
export function semanticScalarName(type) {
  const semantic = new Set(["UUID", "Email", "Password"]);
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

/** Map a TypeSpec type + pointer optionality to Go type string and required imports. */
export function resolveGoType(type, usePtr, imports) {
  if (type.kind === "Scalar") {
    const base = baseScalarName(type);
    switch (base) {
      case "string":
        return "string";
      case "int32":
        return usePtr ? "*int32" : "int32";
      case "int64":
        return usePtr ? "*int64" : "int64";
      case "float64":
        return usePtr ? "*float64" : "float64";
      case "boolean":
        return usePtr ? "*bool" : "bool";
      case "utcDateTime":
      case "offsetDateTime":
        imports.add("time");
        return usePtr ? "*time.Time" : "time.Time";
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
    return usePtr ? `*${type.name}` : type.name;
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
 * (e.g. `BookID string \`json:"-"\`` on ReplaceBookRequest).
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
    const pathGoType = pf.param.type.kind === "Enum" ? pf.param.type.name : "string";
    lines.push(`\t${toGoName(pf.name)} ${pathGoType} \`json:"-"\``);
  }

  for (const prop of model.properties.values()) {
    const goField = toGoName(prop.name);
    const goType = resolveGoType(prop.type, isOptionalPointer(prop), imports);
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
    const vCode = emitValidateMethod(program, model.name, validateProps, imports, extraDecls, validatedModels);
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
    const goType = resolveGoType(prop.type, isOptionalPointer(prop), imports);
    lines.push(`\t${goField} ${goType}`);
  }
  lines.push("}");

  if (emitValidate) {
    const vCode = emitValidateMethod(program, structName, props, imports, extraDecls, validatedModels);
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
 * Always emitted for request structs (and nested request models) so hand-written validateExtra hooks work uniformly.
 */
export function emitValidateMethod(program, structName, props, imports, extraDecls = [], validatedModels = new Set()) {
  const checks = [];

  for (const prop of props) {
    const goField = toGoName(prop.name);
    const jsonName = prop.name;
    const fieldLit = JSON.stringify(jsonName);
    const semantic = semanticScalarName(prop.type);
    const minLen = getMinLength(program, prop) ?? (prop.type.kind === "Scalar" ? getMinLength(program, prop.type) : undefined);
    const maxLen = getMaxLength(program, prop) ?? (prop.type.kind === "Scalar" ? getMaxLength(program, prop.type) : undefined);
    const pattern = getPattern(program, prop) ?? (prop.type.kind === "Scalar" ? getPattern(program, prop.type) : undefined);
    const minVal = getMinValue(program, prop) ?? (prop.type.kind === "Scalar" ? getMinValue(program, prop.type) : undefined);
    const maxVal = getMaxValue(program, prop) ?? (prop.type.kind === "Scalar" ? getMaxValue(program, prop.type) : undefined);
    const minItems = getMinItems(program, prop);
    const maxItems = getMaxItems(program, prop);
    const optionalStrGuard = prop.optional ? `r.${goField} != ""` : "";

    if (semantic === "UUID") {
      emitCheckChain(checks, [`v.UUID(${fieldLit}, r.${goField})`], optionalStrGuard);
      continue;
    }

    if (semantic === "Email") {
      const preds = [`v.Email(${fieldLit}, r.${goField})`];
      if (maxLen !== undefined) {
        preds.push(`v.MaxLength(${fieldLit}, r.${goField}, ${maxLen})`);
      }
      emitCheckChain(checks, preds, optionalStrGuard);
      continue;
    }

    if (semantic === "Password") {
      const preds = [];
      if (!prop.optional) {
        preds.push(`v.Required(${fieldLit}, r.${goField})`);
      }
      if (minLen !== undefined && (prop.optional ? minLen >= 1 : minLen > 1)) {
        preds.push(`v.MinLength(${fieldLit}, r.${goField}, ${minLen})`);
      }
      if (maxLen !== undefined) {
        // bcrypt enforces a strict UTF-8 byte ceiling (72 bytes).
        preds.push(`v.MaxBytes(${fieldLit}, r.${goField}, ${maxLen})`);
      }
      preds.push(`v.PasswordComplexity(${fieldLit}, r.${goField})`);
      emitCheckChain(checks, preds, optionalStrGuard);
      continue;
    }

    const enumVals = stringLiteralValues(prop.type);
    if (enumVals) {
      const expr = prop.type.kind === "Enum" ? `string(r.${goField})` : `r.${goField}`;
      const allowedArgs = enumVals.map((v) => JSON.stringify(v)).join(", ");
      const guard = prop.optional ? `${expr} != ""` : "";
      emitCheckChain(checks, [`validator.OneOf(&v, ${fieldLit}, ${expr}, ${allowedArgs})`], guard);
      continue;
    }

    if (prop.type.kind === "Model") {
      if (prop.type.name === "Array" && prop.type.indexer) {
        emitArrayChecks(checks, {
          goField,
          jsonName,
          fieldLit,
          optional: prop.optional,
          minItems,
          maxItems,
          elemType: prop.type.indexer.value,
          validatedModels,
          imports,
        });
        continue;
      }
      if (prop.type.name !== "Record" && validatedModels.has(prop.type.name)) {
        if (isOptionalPointer(prop)) {
          checks.push(`\tif r.${goField} != nil {`, `\t\tv.Merge(${fieldLit}, r.${goField}.Validate())`, `\t}`);
        } else {
          checks.push(`\tv.Merge(${fieldLit}, r.${goField}.Validate())`);
        }
        continue;
      }
    }

    const base = prop.type.kind === "Scalar" ? baseScalarName(prop.type) : "";

    if (base === "string") {
      emitStringChecks(checks, { structName, goField, fieldLit, optional: prop.optional, minLen, maxLen, pattern, imports, extraDecls });
    }

    if (base === "float64" || base === "int32" || base === "int64") {
      const isPtr = isOptionalPointer(prop);
      emitNumericChecks(checks, { goField, fieldLit, isPtr, minVal, maxVal });
    }
  }

  imports.add("validator");
  return [
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

function emitArrayChecks(checks, { goField, jsonName, fieldLit, optional, minItems, maxItems, elemType, validatedModels, imports }) {
  if (minItems !== undefined) {
    if (optional) {
      checks.push(`\tif r.${goField} != nil {`, `\t\tv.MinItems(${fieldLit}, len(r.${goField}), ${minItems})`, `\t}`);
    } else {
      checks.push(`\tv.MinItems(${fieldLit}, len(r.${goField}), ${minItems})`);
    }
  }
  if (maxItems !== undefined) {
    checks.push(`\tv.MaxItems(${fieldLit}, len(r.${goField}), ${maxItems})`);
  }

  if (elemType.kind === "Model" && elemType.name !== "Array" && elemType.name !== "Record" && validatedModels.has(elemType.name)) {
    imports.add("strconv");
    checks.push(
      `\tfor i := range r.${goField} {`,
      `\t\tv.Merge(${JSON.stringify(jsonName + "[")} + strconv.Itoa(i) + "]", r.${goField}[i].Validate())`,
      `\t}`,
    );
    return;
  }

  const elemSemantic = semanticScalarName(elemType);
  if (elemSemantic === "UUID" || elemSemantic === "Email") {
    imports.add("strconv");
    const method = elemSemantic === "UUID" ? "UUID" : "Email";
    checks.push(
      `\tfor i, item := range r.${goField} {`,
      `\t\tv.${method}(${JSON.stringify(jsonName + "[")} + strconv.Itoa(i) + "]", item)`,
      `\t}`,
    );
    return;
  }

  const elemEnumVals = stringLiteralValues(elemType);
  if (elemEnumVals) {
    imports.add("strconv");
    const expr = elemType.kind === "Enum" ? "string(item)" : "item";
    const allowedArgs = elemEnumVals.map((val) => JSON.stringify(val)).join(", ");
    checks.push(
      `\tfor i, item := range r.${goField} {`,
      `\t\tvalidator.OneOf(&v, ${JSON.stringify(jsonName + "[")} + strconv.Itoa(i) + "]", ${expr}, ${allowedArgs})`,
      `\t}`,
    );
  }
}

function emitCheckChain(checks, preds, optionalGuard = "") {
  if (preds.length === 0) return;
  if (optionalGuard) {
    if (preds.length === 1) {
      checks.push(`\tif ${optionalGuard} {`, `\t\t${preds[0]}`, `\t}`);
      return;
    }
    const cond = [optionalGuard, ...preds.slice(0, -1)].join(" && ");
    checks.push(`\tif ${cond} {`, `\t\t${preds[preds.length - 1]}`, `\t}`);
    return;
  }
  if (preds.length === 1) {
    checks.push(`\t${preds[0]}`);
    return;
  }
  const cond = preds.slice(0, -1).join(" && ");
  checks.push(`\tif ${cond} {`, `\t\t${preds[preds.length - 1]}`, `\t}`);
}

function emitStringChecks(checks, { structName, goField, fieldLit, optional, minLen, maxLen, pattern, imports, extraDecls }) {
  const preds = [];

  if (!optional && minLen !== undefined && minLen >= 1) {
    preds.push(`v.Required(${fieldLit}, r.${goField})`);
    if (minLen > 1) {
      preds.push(`v.MinLength(${fieldLit}, r.${goField}, ${minLen})`);
    }
  } else if (optional && minLen !== undefined && minLen >= 1) {
    preds.push(`v.MinLength(${fieldLit}, r.${goField}, ${minLen})`);
  }

  if (maxLen !== undefined) {
    preds.push(`v.MaxLength(${fieldLit}, r.${goField}, ${maxLen})`);
  }

  if (pattern !== undefined) {
    imports.add("regexp");
    const varName = `pattern${structName}${goField}`;
    extraDecls.push(`var ${varName} = regexp.MustCompile(${JSON.stringify(pattern)})`);
    preds.push(`v.Pattern(${fieldLit}, r.${goField}, ${varName})`);
  }

  emitCheckChain(checks, preds, optional ? `r.${goField} != ""` : "");
}

function emitNumericChecks(checks, { goField, fieldLit, isPtr, minVal, maxVal }) {
  const valExpr = isPtr ? `*r.${goField}` : `r.${goField}`;
  const guard = isPtr ? `r.${goField} != nil` : "";
  const preds = [];

  if (minVal !== undefined) {
    preds.push(`validator.Min(&v, ${fieldLit}, ${valExpr}, ${minVal})`);
  }
  if (maxVal !== undefined) {
    preds.push(`validator.Max(&v, ${fieldLit}, ${valExpr}, ${maxVal})`);
  }

  emitCheckChain(checks, preds, guard);
}
