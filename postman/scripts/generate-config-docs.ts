/* eslint-disable @typescript-eslint/no-explicit-any -- Zod v4 internal schema tree
   is intentionally accessed as `any` because the check/constraint APIs
   (_zod.def.checks, _zod.def.format, etc.) are undocumented and lack
   stable TypeScript types. This file is a build-time script, not runtime code. */
/**
 * Generates Linea Postman env-var config MDX partials from envLoader.ts + schema.ts.
 *
 * Uses the TypeScript compiler API to statically evaluate envLoader.ts's AST
 * (no hard-coded env var inventory) and walks the real Zod schema tree at runtime
 * for descriptions, types, defaults, and constraints.
 *
 * Outputs one MDX partial per section under docs/_generated/postman/.
 * Run: pnpm --filter @lfdt-lineth/postman run docs:generate
 */

import { mkdirSync, readFileSync, writeFileSync, rmSync } from "node:fs";
import path from "node:path";
import ts from "typescript";
import { ZodObject, ZodOptional, ZodLiteral, ZodUnion, ZodString, ZodNumber, ZodBigInt, ZodBoolean } from "zod";

import { postmanOptionsSchema } from "../src/application/postman/app/config/schema";

// ─── Paths ────────────────────────────────────────────────────────────────────

const ENV_LOADER_PATH = path.join(__dirname, "..", "src", "application", "postman", "app", "config", "envLoader.ts");
const OUTPUT_DIR = path.join(__dirname, "..", "docs", "_generated", "postman");
const WRAPPER_PATH = path.join(__dirname, "..", "docs", "linea-postman-options.mdx");

// ─── Types ────────────────────────────────────────────────────────────────────

type SectionId =
  | "general"
  | "l1-network"
  | "l2-network"
  | "listener"
  | "claiming"
  | "signer"
  | "database"
  | "database-cleaner"
  | "api";

type Row = {
  envVar: string;
  description: string;
  default: string;
  type: string;
  requiredLabel: string;
  section: SectionId;
};

type Params = Record<string, string>;

// ─── Section metadata ─────────────────────────────────────────────────────────

const SECTION_TITLES: Record<SectionId, string> = {
  general: "General",
  "l1-network": "L1 network",
  "l2-network": "L2 network",
  listener: "Listener",
  claiming: "Claiming",
  signer: "Signer",
  database: "Database",
  "database-cleaner": "Database cleaner",
  api: "API",
};

const SECTION_ORDER: SectionId[] = [
  "general",
  "l1-network",
  "l2-network",
  "listener",
  "claiming",
  "signer",
  "database",
  "database-cleaner",
  "api",
];

// ─── Manual overrides (schema doesn't carry enough info) ──────────────────────

/**
 * loggerOptions is z.any().optional() — no shape, no type info.
 * envLoader sets LOG_LEVEL as a string with default "info".
 */
const LOG_LEVEL_ROW: Row = {
  envVar: "LOG_LEVEL",
  description: "Log level for the Winston logger",
  default: "`info`",
  type: "string",
  requiredLabel: "optional",
  section: "general",
};

/**
 * databaseOptions is z.object().passthrough() — Postgres connection fields
 * aren't enumerated in the Zod schema and must be listed by hand.
 * Descriptions and types are intentionally blank (matching the old tool's output)
 * because the passthrough schema carries no metadata for these fields.
 */
const POSTGRES_ROWS: Row[] = [
  {
    envVar: "POSTGRES_DB",
    description: "",
    default: "`postman_db`",
    type: "",
    requiredLabel: "optional",
    section: "database",
  },
  { envVar: "POSTGRES_HOST", description: "", default: "—", type: "", requiredLabel: "optional", section: "database" },
  {
    envVar: "POSTGRES_PASSWORD",
    description: "",
    default: "—",
    type: "",
    requiredLabel: "optional",
    section: "database",
  },
  {
    envVar: "POSTGRES_PORT",
    description: "",
    default: "`5432`",
    type: "number",
    requiredLabel: "optional",
    section: "database",
  },
  {
    envVar: "POSTGRES_SSL",
    description: "",
    default: "`false`",
    type: "boolean",
    requiredLabel: 'optional; when "true", ssl object is included',
    section: "database",
  },
  {
    envVar: "POSTGRES_SSL_CA_PATH",
    description: "",
    default: "—",
    type: "",
    requiredLabel: "optional; only when POSTGRES_SSL=true",
    section: "database",
  },
  {
    envVar: "POSTGRES_SSL_REJECT_UNAUTHORIZED",
    description: "",
    default: "`false`",
    type: "boolean",
    requiredLabel: "optional; only when POSTGRES_SSL=true",
    section: "database",
  },
  { envVar: "POSTGRES_USER", description: "", default: "—", type: "", requiredLabel: "optional", section: "database" },
];

/**
 * Conditional inclusion notes from envLoader.ts that can't be derived
 * from the Zod schema (spread/conditional logic in the source).
 */
const CONDITIONAL_NOTES: Record<string, string> = {
  "l1Options.listener.initialFromBlock": "included only when parseInt(value) >= 0",
  "l2Options.listener.initialFromBlock": "included only when parseInt(value) >= 0",
  "l1Options.listener.blockConfirmation": "included only when parseInt(value) >= 0",
  "l2Options.listener.blockConfirmation": "included only when parseInt(value) >= 0",
  "l1Options.listener.eventFilters.fromAddressFilter": "eventFilters object included only when any filter is set",
  "l2Options.listener.eventFilters.fromAddressFilter": "eventFilters object included only when any filter is set",
  "l1Options.listener.eventFilters.toAddressFilter": "eventFilters object included only when any filter is set",
  "l2Options.listener.eventFilters.toAddressFilter": "eventFilters object included only when any filter is set",
  "l1Options.listener.eventFilters.calldataFilter.criteriaExpression":
    "requires both CALLDATA and CALLDATA_FUNCTION_INTERFACE",
  "l2Options.listener.eventFilters.calldataFilter.criteriaExpression":
    "requires both CALLDATA and CALLDATA_FUNCTION_INTERFACE",
  "l1Options.listener.eventFilters.calldataFilter.calldataFunctionInterface":
    "requires both CALLDATA and CALLDATA_FUNCTION_INTERFACE",
  "l2Options.listener.eventFilters.calldataFilter.calldataFunctionInterface":
    "requires both CALLDATA and CALLDATA_FUNCTION_INTERFACE",
  "l1Options.claiming.signer.tls.keyStorePath": "TLS object included only when this path is set",
  "l2Options.claiming.signer.tls.keyStorePath": "TLS object included only when this path is set",
  "l1Options.claiming.signer.tls.keyStorePassword": "included with TLS object",
  "l2Options.claiming.signer.tls.keyStorePassword": "included with TLS object",
  "l1Options.claiming.signer.tls.trustStorePath": "included with TLS object",
  "l2Options.claiming.signer.tls.trustStorePath": "included with TLS object",
  "l1Options.claiming.signer.tls.trustStorePassword": "included with TLS object",
  "l2Options.claiming.signer.tls.trustStorePassword": "included with TLS object",
  "l1Options.claiming.signer.region": "included only when set (aws-kms)",
  "l2Options.claiming.signer.region": "included only when set (aws-kms)",
};

// ─── AST-based env var extraction ─────────────────────────────────────────────

/**
 * Statically evaluates envLoader.ts's AST to build a map of config path
 * (e.g. `l1Options.claiming.signer.privateKey`) -> env var name (e.g.
 * `L1_SIGNER_PRIVATE_KEY`). No hard-coded knowledge of individual field names.
 */
function resolveEnvVarsFromSource(): Map<string, string> {
  const source = readFileSync(ENV_LOADER_PATH, "utf8");
  const sourceFile = ts.createSourceFile(ENV_LOADER_PATH, source, ts.ScriptTarget.Latest, true);

  const functions = new Map<string, ts.FunctionDeclaration>();
  let loadFn: ts.FunctionDeclaration | undefined;
  ts.forEachChild(sourceFile, (node) => {
    if (ts.isFunctionDeclaration(node) && node.name) {
      if (node.name.text === "loadPostmanOptionsFromEnv") loadFn = node;
      else functions.set(node.name.text, node);
    }
  });
  if (!loadFn?.body) throw new Error(`Could not find loadPostmanOptionsFromEnv in ${ENV_LOADER_PATH}`);

  const envVarByPath = new Map<string, string>();

  const substitute = (text: string, params: Params): string =>
    Object.entries(params).reduce((out, [key, value]) => out.replaceAll(`\${${key}}`, value), text);

  function bindLocalConditionalParams(fn: ts.FunctionDeclaration, params: Params): Params {
    const bound: Params = { ...params };
    if (!fn.body) return bound;
    for (const stmt of fn.body.statements) {
      if (!ts.isVariableStatement(stmt)) continue;
      for (const decl of stmt.declarationList.declarations) {
        if (!ts.isIdentifier(decl.name) || !decl.initializer || !ts.isConditionalExpression(decl.initializer)) {
          continue;
        }
        const { condition, whenTrue, whenFalse } = decl.initializer;
        if (!ts.isBinaryExpression(condition) || !ts.isStringLiteral(condition.right)) continue;
        if (!ts.isIdentifier(condition.left)) continue;
        const lhsValue = bound[condition.left.text];
        if (!ts.isStringLiteral(whenTrue) || !ts.isStringLiteral(whenFalse)) continue;
        bound[decl.name.text] = lhsValue === condition.right.text ? whenTrue.text : whenFalse.text;
      }
    }
    return bound;
  }

  function resolveArg(argExpr: ts.Expression, params: Params): string {
    if (ts.isStringLiteral(argExpr)) return argExpr.text;
    if (ts.isIdentifier(argExpr)) return params[argExpr.text] ?? argExpr.text;
    return substitute(argExpr.getText(), params);
  }

  function findEnvVarRefs(expr: ts.Node, params: Params): string[] {
    const found: string[] = [];
    const visit = (node: ts.Node) => {
      if (ts.isPropertyAccessExpression(node) && node.expression.getText() === "process.env") {
        found.push(node.name.text);
      } else if (
        ts.isElementAccessExpression(node) &&
        node.expression.getText() === "process.env" &&
        (ts.isTemplateExpression(node.argumentExpression) ||
          ts.isNoSubstitutionTemplateLiteral(node.argumentExpression))
      ) {
        const raw = node.argumentExpression.getText().slice(1, -1);
        found.push(substitute(raw, params));
      }
      ts.forEachChild(node, visit);
    };
    visit(expr);
    return found;
  }

  function unwrap(expr: ts.Expression): ts.Expression {
    if (ts.isParenthesizedExpression(expr)) return unwrap(expr.expression);
    if (ts.isAsExpression(expr) || ts.isNonNullExpression(expr)) return unwrap(expr.expression);
    return expr;
  }

  function resolveLocalDeclaration(name: string, scope: ts.Block | undefined): ts.Expression | undefined {
    if (!scope) return undefined;
    let found: ts.Expression | undefined;
    const visit = (node: ts.Node) => {
      if (found) return;
      if (ts.isVariableDeclaration(node) && ts.isIdentifier(node.name) && node.name.text === name && node.initializer) {
        found = node.initializer;
        return;
      }
      ts.forEachChild(node, visit);
    };
    visit(scope);
    return found;
  }

  function walkObject(obj: ts.ObjectLiteralExpression, currentPath: string, params: Params, scope?: ts.Block): void {
    for (const prop of obj.properties) {
      if (ts.isPropertyAssignment(prop)) {
        handleValue(
          prop.initializer,
          currentPath ? `${currentPath}.${prop.name.getText()}` : prop.name.getText(),
          params,
          scope,
        );
      } else if (ts.isSpreadAssignment(prop)) {
        handleSpreadOrCall(prop.expression, currentPath, params, scope);
      } else if (ts.isShorthandPropertyAssignment(prop)) {
        handleValue(prop.name, currentPath ? `${currentPath}.${prop.name.text}` : prop.name.text, params, scope);
      }
    }
  }

  function handleSpreadOrCall(expr: ts.Expression, currentPath: string, params: Params, scope?: ts.Block): void {
    const inner = unwrap(expr);
    if (ts.isConditionalExpression(inner)) {
      handleSpreadOrCall(inner.whenTrue, currentPath, params, scope);
      return;
    }
    if (ts.isObjectLiteralExpression(inner)) {
      walkObject(inner, currentPath, params, scope);
      return;
    }
    if (ts.isCallExpression(inner)) {
      handleCall(inner, currentPath, params);
    }
  }

  function handleCall(call: ts.CallExpression, currentPath: string, params: Params): boolean {
    const fn = functions.get(call.expression.getText());
    if (!fn?.body) return false;

    const callParams: Params = {};
    fn.parameters.forEach((param, i) => {
      const argExpr = call.arguments[i];
      if (argExpr) callParams[param.name.getText()] = resolveArg(argExpr, params);
    });
    const boundParams = bindLocalConditionalParams(fn, callParams);

    let foundObjectReturn = false;
    const visit = (node: ts.Node) => {
      if (ts.isReturnStatement(node) && node.expression) {
        const returned = unwrap(node.expression);
        if (ts.isObjectLiteralExpression(returned)) {
          foundObjectReturn = true;
          walkObject(returned, currentPath, boundParams, fn.body);
        }
      }
      ts.forEachChild(node, visit);
    };
    visit(fn.body);
    return foundObjectReturn;
  }

  function handleValue(expr: ts.Expression, currentPath: string, params: Params, scope?: ts.Block): void {
    const inner = unwrap(expr);
    if (ts.isObjectLiteralExpression(inner)) {
      walkObject(inner, currentPath, params, scope);
      return;
    }
    if (ts.isCallExpression(inner) && handleCall(inner, currentPath, params)) {
      return;
    }
    if (ts.isIdentifier(inner)) {
      const declaration = resolveLocalDeclaration(inner.text, scope);
      if (declaration) {
        handleValue(declaration, currentPath, params, scope);
        return;
      }
    }
    const envVarRefs = findEnvVarRefs(inner, params);
    if (envVarRefs.length > 0) envVarByPath.set(currentPath, envVarRefs[0]);
  }

  const visitTop = (node: ts.Node) => {
    if (ts.isReturnStatement(node) && node.expression) {
      const returned = unwrap(node.expression);
      if (ts.isObjectLiteralExpression(returned)) walkObject(returned, "", {}, loadFn?.body);
    }
    ts.forEachChild(node, visitTop);
  };
  visitTop(loadFn.body);

  // The signer's `type` is read into a local variable and branched on,
  // so the discriminator env var must be resolved from the helper's declaration.
  const signerConfigFn = functions.get("buildSignerConfig");
  if (signerConfigFn?.body) {
    for (const stmt of signerConfigFn.body.statements) {
      if (!ts.isVariableStatement(stmt)) continue;
      for (const decl of stmt.declarationList.declarations) {
        if (!ts.isIdentifier(decl.name) || !decl.initializer) continue;
        const refs = findEnvVarRefs(decl.initializer, { prefix: "${prefix}" });
        if (refs.length > 0) {
          for (const prefix of ["L1", "L2"]) {
            envVarByPath.set(
              `${prefix.toLowerCase()}Options.claiming.signer.type`,
              refs[0].replace("${prefix}", prefix),
            );
          }
        }
      }
    }
  }

  return envVarByPath;
}

// ─── Section mapping ──────────────────────────────────────────────────────────

const ENV_VAR_BY_PATH = resolveEnvVarsFromSource();

function resolveEnvVar(configPath: string): string | undefined {
  return ENV_VAR_BY_PATH.get(configPath);
}

function sectionForPath(configPath: string): SectionId {
  if (
    configPath === "l1L2AutoClaimEnabled" ||
    configPath === "l2L1AutoClaimEnabled" ||
    configPath === "loggerOptions" ||
    configPath === "loggerOptions.level"
  ) {
    return "general";
  }
  if (configPath === "databaseOptions" || configPath.startsWith("databaseOptions.")) return "database";
  if (configPath === "databaseCleanerOptions" || configPath.startsWith("databaseCleanerOptions.")) {
    return "database-cleaner";
  }
  if (configPath === "apiOptions" || configPath.startsWith("apiOptions.")) return "api";

  const netMatch = configPath.match(/^(l[12])Options\.(.+)$/);
  if (netMatch) {
    const prefix = netMatch[1];
    const sub = netMatch[2];
    if (!sub.includes(".")) return prefix === "l1" ? "l1-network" : "l2-network";
    if (sub === "l2MessageTreeDepth" || sub === "enableLineaEstimateGas") return "l2-network";
    if (sub.startsWith("listener.")) return "listener";
    if (sub.startsWith("claiming.signer.")) return "signer";
    if (sub.startsWith("claiming.")) return "claiming";
  }

  throw new Error(`Could not determine section for config path: ${configPath}`);
}

// ─── Zod schema tree traversal ────────────────────────────────────────────────

function getRequiredKeys(obj: ZodObject): Set<string> {
  const required = new Set<string>();
  for (const [key, val] of Object.entries(obj.shape)) {
    if (!(val instanceof ZodOptional)) required.add(key);
  }
  return required;
}

function describeTypeFromZod(schema: any): string {
  if (schema instanceof ZodString) {
    const checks: any[] = schema._zod?.def?.checks ?? [];
    for (const c of checks) {
      const def: any = c._zod?.def;
      if (def?.format === "url") return "string (url)";
      if (def?.format === "regex") return "address";
    }
    return "string";
  }
  if (schema instanceof ZodNumber) {
    const checks: any[] = schema._zod?.def?.checks ?? [];
    const parts: string[] = [];
    let isInt = false;
    let isPositive = false;
    let isNonnegative = false;
    let minVal: number | undefined;
    let maxVal: number | undefined;
    for (const c of checks) {
      const def: any = c._zod?.def;
      if (def?.format === "safeint") isInt = true;
      if (def?.check === "greater_than") {
        if (def.inclusive === false) isPositive = true;
        else {
          isNonnegative = true;
          if (typeof def.value === "number") minVal = def.value;
        }
      }
      if (def?.check === "less_than" && typeof def.value === "number") maxVal = def.value;
    }
    parts.push(isInt ? "int" : "number");
    if (typeof minVal === "number" && typeof maxVal === "number") {
      parts.push(`min ${minVal}, max ${maxVal}`);
    } else if (isPositive && typeof maxVal === "number") {
      parts.push("positive");
      parts.push(`max ${maxVal}`);
    } else if (isPositive) {
      parts.push("positive");
    } else if (isNonnegative) {
      parts.push("nonnegative");
    } else if (typeof minVal === "number") {
      parts.push(`min ${minVal}`);
    } else if (typeof maxVal === "number") {
      parts.push(`max ${maxVal}`);
    }
    // Format: "number (int, positive, max 65535)" or "number (positive)" or "number"
    if (parts.length > 1) {
      // parts[0] is "int" or "number"; constraints are parts[1:]
      // When int, include it: "number (int, positive, max 65535)"
      // When not int: "number (positive)"
      const constraints = parts.slice(1);
      if (isInt) {
        return `number (int, ${constraints.join(", ")})`;
      }
      return `number (${constraints.join(", ")})`;
    }
    // If only "int" with no constraints, return "number"
    return parts[0] === "int" ? "number" : parts[0];
  }
  if (schema instanceof ZodBigInt) {
    const checks = schema._zod?.def?.checks ?? [];
    for (const c of checks) {
      if (c._zod?.def?.check === "greater_than") return "bigint (positive)";
    }
    return "bigint";
  }
  if (schema instanceof ZodBoolean) return "boolean";
  if (schema instanceof ZodLiteral) return "string";
  // z.custom() — use description to detect specific types
  const desc = schema.description ?? "";
  if (desc.includes("Hex-encoded 32-byte private key")) return "hex private key (32 bytes)";
  if (desc.includes("Hex string starting with 0x")) return "hex string";
  return "string";
}

function buildRow(
  schema: any,
  currentPath: string,
  isRequired: boolean,
  contextNote: string,
  descriptionFromOptional?: string,
): Row | undefined {
  const envVar = resolveEnvVar(currentPath);
  if (!envVar) return undefined;

  // Prefer the field-specific description from the ZodOptional wrapper,
  // fall back to the inner schema description (e.g. ethAddress's generic desc)
  const description = descriptionFromOptional ?? schema.description ?? "";
  const conditionalNote = CONDITIONAL_NOTES[currentPath];
  const effectiveNote = conditionalNote ?? contextNote;
  const requiredLabel = effectiveNote ? `optional; ${effectiveNote}` : isRequired ? "required" : "optional";

  let typeStr = describeTypeFromZod(schema);
  if (description.includes("Hex-encoded 32-byte private key")) typeStr = "hex private key (32 bytes)";
  if (description.includes("Public key of the Web3Signer")) typeStr = "hex string";
  if (description.includes("Ethereum address") || description.includes("message service contract")) typeStr = "address";
  if (description.includes("JSON-RPC endpoint URL")) typeStr = "string (url)";

  // Defaults
  let defaultStr = "—";
  if (schema instanceof ZodBoolean) {
    defaultStr = "`false`";
  } else if (envVar.endsWith("_SIGNER_TYPE")) {
    defaultStr = "`private-key`";
  } else if (envVar === "LOG_LEVEL") {
    defaultStr = "`info`";
  }

  // Secret/placeholder vars always have blank defaults
  if (
    envVar.endsWith("_RPC_URL") ||
    envVar.endsWith("_CONTRACT_ADDRESS") ||
    envVar.endsWith("_PRIVATE_KEY") ||
    envVar.endsWith("_WEB3_SIGNER_PUBLIC_KEY") ||
    envVar.endsWith("_WEB3_SIGNER_ENDPOINT") ||
    envVar.endsWith("_AWS_KMS_KEY_ID") ||
    envVar.endsWith("_TLS_KEYSTORE_PASSWORD") ||
    envVar.endsWith("_TLS_TRUSTSTORE_PATH") ||
    envVar.endsWith("_TLS_TRUSTSTORE_PASSWORD") ||
    envVar.endsWith("_TLS_KEYSTORE_PATH") ||
    envVar.endsWith("_CLAIM_VIA_ADDRESS") ||
    envVar.endsWith("_EVENT_FILTER_FROM_ADDRESS") ||
    envVar.endsWith("_EVENT_FILTER_TO_ADDRESS")
  ) {
    defaultStr = "—";
  }

  return {
    envVar,
    description,
    default: defaultStr,
    type: typeStr,
    requiredLabel,
    section: sectionForPath(currentPath),
  };
}

function collectRows(
  schema: any,
  currentPath: string,
  contextNote: string,
  rows: Row[],
  descriptionFromOptional?: string,
): void {
  if (schema instanceof ZodOptional) {
    const wrapperDesc = schema.description ?? descriptionFromOptional;
    collectRows(schema.unwrap(), currentPath, contextNote, rows, wrapperDesc);
    return;
  }

  if (schema instanceof ZodObject) {
    const requiredKeys = getRequiredKeys(schema);
    for (const [key, childSchema] of Object.entries(schema.shape)) {
      const childPath = currentPath ? `${currentPath}.${key}` : key;
      const isRequired = requiredKeys.has(key) && !contextNote;
      collectLeafOrRecurse(childSchema as any, childPath, isRequired, contextNote, rows);
    }
    return;
  }

  if (currentPath === "loggerOptions") {
    rows.push(LOG_LEVEL_ROW);
    return;
  }

  if (schema instanceof ZodUnion) {
    collectDiscriminatedUnion(schema.options as any[], currentPath, contextNote, rows);
    return;
  }

  const row = buildRow(schema, currentPath, true, contextNote, descriptionFromOptional);
  if (row) rows.push(row);
}

function collectDiscriminatedUnion(variants: any[], currentPath: string, contextNote: string, rows: Row[]): void {
  let discriminatorKey: string | undefined;
  for (const [key, val] of Object.entries(variants[0].shape)) {
    if (val instanceof ZodLiteral) {
      discriminatorKey = key;
      break;
    }
  }

  if (discriminatorKey) {
    const discriminatorEnvVar = resolveEnvVar(`${currentPath}.${discriminatorKey}`);
    const allowedValues = variants
      .map((v) =>
        v.shape[discriminatorKey!] instanceof ZodLiteral ? (v.shape[discriminatorKey!].value as string) : undefined,
      )
      .filter((v: unknown): v is string => v !== undefined);
    const firstDescription = variants.map((v) => v.shape[discriminatorKey!]?.description).find(Boolean);

    if (discriminatorEnvVar) {
      let mergedDesc = firstDescription ?? "";
      if (discriminatorKey === "type" && currentPath.endsWith("claiming.signer")) {
        mergedDesc = 'Signer backend type: "private-key" (local key), "web3signer" (remote), or "aws-kms"';
      }
      rows.push({
        envVar: discriminatorEnvVar,
        description: mergedDesc,
        default: discriminatorKey === "type" && currentPath.endsWith("claiming.signer") ? "`private-key`" : "—",
        type: `string (${allowedValues.join("|")})`,
        requiredLabel: `required; allowed: ${allowedValues.join("|")}`,
        section: sectionForPath(`${currentPath}.${discriminatorKey}`),
      });
    }
  }

  for (const variant of variants) {
    const constValue =
      discriminatorKey && variant.shape[discriminatorKey] instanceof ZodLiteral
        ? (variant.shape[discriminatorKey].value as string)
        : undefined;
    const discriminatorEnvVar = discriminatorKey ? resolveEnvVar(`${currentPath}.${discriminatorKey}`) : undefined;
    const note =
      discriminatorKey && constValue !== undefined
        ? `used when ${discriminatorEnvVar ?? `${currentPath}.${discriminatorKey}`} is "${constValue}"`
        : contextNote;

    for (const [key, childSchema] of Object.entries(variant.shape)) {
      if (key === discriminatorKey) continue;
      const childPath = currentPath ? `${currentPath}.${key}` : key;
      const inner = childSchema instanceof ZodOptional ? childSchema.unwrap() : childSchema;
      if (inner instanceof ZodObject) {
        collectRows(childSchema as any, childPath, note, rows);
      } else {
        collectLeafOrRecurse(childSchema as any, childPath, false, note, rows);
      }
    }
  }
}

function collectLeafOrRecurse(
  schema: any,
  currentPath: string,
  isRequired: boolean,
  contextNote: string,
  rows: Row[],
): void {
  let descriptionFromOptional: string | undefined;
  let inner = schema;
  let actualIsRequired = isRequired;
  if (inner instanceof ZodOptional) {
    actualIsRequired = false;
    descriptionFromOptional = inner.description;
    inner = inner.unwrap();
  }

  if (inner instanceof ZodObject) {
    collectRows(inner, currentPath, contextNote, rows, descriptionFromOptional);
    return;
  }
  if (inner instanceof ZodUnion) {
    collectRows(inner, currentPath, contextNote, rows, descriptionFromOptional);
    return;
  }

  // loggerOptions is z.any() after unwrapping — emit LOG_LEVEL manually
  if (currentPath === "loggerOptions") {
    rows.push(LOG_LEVEL_ROW);
    return;
  }

  const row = buildRow(inner, currentPath, actualIsRequired, contextNote, descriptionFromOptional);
  if (row) rows.push(row);
}

// ─── Dedup and sort ───────────────────────────────────────────────────────────

function dedupeAndSortRows(rows: Row[]): Row[] {
  const seen = new Map<string, Row>();
  for (const row of rows) {
    if (!seen.has(row.envVar)) seen.set(row.envVar, row);
  }
  return [...seen.values()].sort((a, b) => a.envVar.localeCompare(b.envVar));
}

// ─── MDX rendering ────────────────────────────────────────────────────────────

function escapeCell(value: string): string {
  let out = value.replace(/\r?\n/g, " ").replace(/\s+/g, " ").trim();
  out = out.replace(/\\/g, "\\\\");
  out = out.replace(/`/g, "&#96;");
  out = out.replace(/\{/g, "\\{").replace(/\}/g, "\\}");
  out = out.replace(/</g, "&lt;").replace(/>/g, "&gt;");
  out = out.replace(/\|/g, "\\|");
  return out;
}

function escapeInlineCode(text: string): string {
  let out = text.replace(/\r?\n/g, " ").replace(/\s+/g, " ").trim();
  out = out.replace(/\\/g, "\\\\");
  out = out.replace(/`/g, "&#96;");
  out = out.replace(/\{/g, "\\{").replace(/\}/g, "\\}");
  out = out.replace(/\|/g, "\\|");
  return out;
}

function renderSectionPartial(sectionId: SectionId, rows: Row[]): string {
  const sectionRows = rows.filter((r) => r.section === sectionId);
  if (sectionRows.length === 0) return "";

  const lines: string[] = [];
  lines.push(`### ${SECTION_TITLES[sectionId]}`);
  lines.push("");
  lines.push("| Env var | Description | Default | Type | Required |");
  lines.push("| --- | --- | --- | --- | --- |");
  for (const row of sectionRows) {
    const cells = [
      `\`${escapeInlineCode(row.envVar)}\``,
      escapeCell(row.description),
      row.default,
      row.type ? `\`${escapeInlineCode(row.type)}\`` : "",
      escapeCell(row.requiredLabel),
    ];
    lines.push(`| ${cells.join(" | ")} |`);
  }
  lines.push("");
  return lines.join("\n");
}

function partialComponentName(sectionId: SectionId): string {
  return sectionId
    .split(/[^A-Za-z0-9]+/)
    .filter(Boolean)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join("");
}

function renderWrapper(partials: { sectionId: SectionId; componentName: string }[]): string {
  const lines: string[] = [];
  lines.push("---");
  lines.push("title: Linea Postman configuration");
  lines.push("slug: /reference/component-configuration/linea-postman-options");
  lines.push("description: Auto-generated reference of Linea Postman environment variables, grouped by section.");
  lines.push("draft: false");
  lines.push("---");
  lines.push("");
  lines.push(
    "{/* Human-owned wrapper. Automation only updates `_generated/postman/` partials. " +
      "Seeded once by generate-config-docs.ts; place new partial imports when sections appear. */}",
  );
  lines.push("");
  lines.push('import Provenance from "./_generated/postman/provenance.mdx";');
  for (const p of partials) {
    lines.push(`import ${p.componentName} from "./_generated/postman/${p.sectionId}.mdx";`);
  }
  lines.push("");
  lines.push(
    "This reference lists Linea Postman environment variables, grouped by section. " +
      "Variable names and defaults come from `envLoader.ts`; descriptions and types come from " +
      "Zod `.describe()` on `schema.ts`. Secrets and endpoints are documented by name only — " +
      "never with live values.",
  );
  lines.push("");
  lines.push("<Provenance />");
  lines.push("");
  for (const p of partials) {
    lines.push(`<${p.componentName} />`);
    lines.push("");
  }
  return lines.join("\n").replace(/\s+$/, "") + "\n";
}

// ─── Exports for drift checker ────────────────────────────────────────────────

export type PartialResult = { sectionId: SectionId; componentName: string; markdown: string };

export function generatePartials(): { partials: PartialResult[]; wrapperMarkdown: string } {
  const rows: Row[] = [];
  collectRows(postmanOptionsSchema as any, "", "", rows);
  rows.push(...POSTGRES_ROWS);
  const deduped = dedupeAndSortRows(rows);

  const partials: PartialResult[] = [];
  for (const sectionId of SECTION_ORDER) {
    const markdown = renderSectionPartial(sectionId, deduped);
    if (markdown) {
      partials.push({ sectionId, componentName: partialComponentName(sectionId), markdown });
    }
  }

  return { partials, wrapperMarkdown: renderWrapper(partials) };
}

// ─── Main ─────────────────────────────────────────────────────────────────────

function main(): void {
  const { partials, wrapperMarkdown } = generatePartials();

  rmSync(OUTPUT_DIR, { recursive: true, force: true });
  mkdirSync(OUTPUT_DIR, { recursive: true });

  for (const p of partials) {
    writeFileSync(path.join(OUTPUT_DIR, `${p.sectionId}.mdx`), p.markdown);
  }
  writeFileSync(WRAPPER_PATH, wrapperMarkdown);

  console.log(`Generated ${partials.length} Postman config partials.`);
  console.log(`  output: ${path.relative(process.cwd(), OUTPUT_DIR)}`);
  for (const p of partials) {
    console.log(`    - ${p.sectionId}.mdx`);
  }
  console.log(`  wrapper: ${path.relative(process.cwd(), WRAPPER_PATH)}`);
}

if (require.main === module) {
  main();
}
