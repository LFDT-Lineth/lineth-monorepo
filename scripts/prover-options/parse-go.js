/**
 * Static Go parser for prover/config TOML keys.
 * No Go compile — reads struct tags + viper.SetDefault from source text.
 */

const fs = require("node:fs");
const path = require("node:path");

/** Section key → partial filename (without .mdx) and display title. */
const SECTION_META = [
  { id: "top-level", title: "Top-level", match: (k) => !k.includes(".") },
  { id: "controller", title: "Controller", match: (k) => k.startsWith("controller.") },
  { id: "execution", title: "Execution", match: (k) => k.startsWith("execution.") },
  {
    id: "data-availability",
    title: "Data availability",
    match: (k) => k.startsWith("data_availability."),
  },
  { id: "invalidity", title: "Invalidity", match: (k) => k.startsWith("invalidity.") },
  { id: "aggregation", title: "Aggregation", match: (k) => k.startsWith("aggregation.") },
  {
    id: "public-input-interconnection",
    title: "Public input interconnection",
    match: (k) => k.startsWith("public_input_interconnection."),
  },
  { id: "debug", title: "Debug", match: (k) => k.startsWith("debug.") },
  { id: "layer2", title: "Layer2", match: (k) => k.startsWith("layer2.") },
  { id: "traces-limits", title: "Traces limits", match: () => false, noteOnly: true },
];

/** Fields excluded even when they would otherwise get a mapstructure/viper key. */
const HARD_EXCLUSIONS = [
  { typeName: "Controller", field: "LocalID", reason: "env-only/CLI (--local-id), not TOML" },
  { typeName: "PublicInput", field: "MockKeccakWizard", reason: "not serialized (testing only)" },
  { typeName: "PublicInput", field: "ChainID", reason: "filled post-load from layer2" },
  { typeName: "PublicInput", field: "BaseFee", reason: "filled post-load from layer2" },
  { typeName: "PublicInput", field: "CoinBase", reason: "filled post-load from layer2" },
  { typeName: "PublicInput", field: "L2MsgServiceAddr", reason: "filled post-load from layer2" },
  {
    typeName: "PublicInput",
    field: "IsAllowedCircuitID",
    reason: "filled post-load from aggregation",
  },
  { typeName: "TracesLimits", field: "ToLargeMode", reason: "runtime-only" },
  { typeName: "TracesLimits", field: "ScalingFactor", reason: "runtime-only" },
];

const TRACES_LIMITS_NOTE = [
  "The `traces_limits` section configures per-arithmetization-module row budgets used during execution proving.",
  "",
  "It is a list of module entries under `traces_limits.modules`, each with:",
  "",
  "- `module` (string, required) — module name prefix; the empty string is the default fallback",
  "- `limit` (int, required, power of 2) — limit used in normal mode",
  "- `limit_large` (int, required, power of 2) — limit used when large mode is active",
  "- `is_not_scalable` (bool, optional) — when true, limitless scaling must not multiply this entry",
  "",
  "Lookup uses longest-prefix matching after sorting entries in reverse alphabetical order.",
  "Large mode (for example via `--large`) selects `limit_large` instead of `limit`.",
  "When a scaling factor is applied and `is_not_scalable` is false, the selected limit is multiplied.",
  "",
  "Concrete module tables are environment-specific; see the prover sample `config-*.toml` files for examples.",
  "Those samples are not portable defaults and are not reproduced here.",
].join("\n");

function isDevNoiseComment(line) {
  const t = line.trim();
  if (!t) return true;
  if (/^TODO\b/i.test(t)) return true;
  if (/^FIXME\b/i.test(t)) return true;
  if (/^XXX\b/i.test(t)) return true;
  // Line-start only — mid-sentence @gbotrel must not drop useful text (see cleanDescription).
  if (/^@gbotrel\b/i.test(t)) return true;
  if (/not serialized/i.test(t)) return true;
  if (/for testing purposes only/i.test(t)) return true;
  if (/^duplicate from\b/i.test(t)) return true;
  if (/^note @/i.test(t)) return true;
  if (/we should remove them/i.test(t)) return true;
  if (/the only reason we keep these is for test/i.test(t)) return true;
  return false;
}

function scrubGbotrelSentences(line) {
  // Drop sentences that mention @gbotrel; keep the rest of the line.
  const sentences = line.split(/(?<=\.)\s+/);
  return sentences
    .filter((s) => !/@gbotrel\b/i.test(s))
    .join(" ")
    .replace(/\s+/g, " ")
    .trim();
}

function cleanDescription(commentLines) {
  const kept = [];
  for (const raw of commentLines) {
    let line = String(raw)
      .replace(/^\s*\/\/\s?/, "")
      .trim();
    if (isDevNoiseComment(line)) continue;
    line = scrubGbotrelSentences(line);
    if (!line || isDevNoiseComment(line)) continue;
    kept.push(line);
  }
  return kept.join(" ").replace(/\s+/g, " ").trim();
}

function parseMapstructure(tag) {
  if (!tag) return { present: false };
  const m = tag.match(/mapstructure:"([^"]*)"/);
  if (!m) return { present: false };
  const raw = m[1];
  if (raw === "-") return { present: true, skip: true };
  const parts = raw.split(",");
  const name = parts[0];
  // ",squash" alone → name empty + squash
  if (raw === ",squash" || (name === "" && parts.includes("squash"))) {
    return { present: true, squash: true, name: "" };
  }
  if (parts.includes("squash")) {
    return { present: true, squash: true, name: name || "" };
  }
  return { present: true, name: name || "", squash: false };
}

function parseValidate(tag) {
  if (!tag) return { required: false, oneof: null, constraints: [] };
  const m = tag.match(/validate:"([^"]*)"/);
  if (!m) return { required: false, oneof: null, constraints: [] };
  const parts = m[1]
    .split(",")
    .map((p) => p.trim())
    .filter(Boolean);
  const required = parts.includes("required");
  let oneof = null;
  const constraints = [];
  for (const p of parts) {
    if (p.startsWith("oneof=")) {
      oneof = p.slice("oneof=".length).split(/\s+/).filter(Boolean);
    } else if (p !== "required" && p !== "dive" && p !== "number") {
      constraints.push(p);
    }
  }
  return { required, oneof, constraints };
}

function extractStructBodies(source) {
  const structs = new Map();
  const re = /type\s+(\w+)\s+struct\s*\{/g;
  let m;
  while ((m = re.exec(source)) !== null) {
    const name = m[1];
    const start = m.index + m[0].length;
    let depth = 1;
    let i = start;
    while (i < source.length && depth > 0) {
      const ch = source[i];
      if (ch === "{") depth++;
      else if (ch === "}") depth--;
      i++;
    }
    structs.set(name, source.slice(start, i - 1));
  }
  return structs;
}

function findMatchingBrace(text, openIdx) {
  let depth = 0;
  for (let i = openIdx; i < text.length; i++) {
    if (text[i] === "{") depth++;
    else if (text[i] === "}") {
      depth--;
      if (depth === 0) return i;
    }
  }
  return -1;
}

/**
 * Parse fields from a struct body string.
 * Returns { name, typeName, tags, comments, anonymousBody? }
 */
function parseStructFields(body) {
  const fields = [];
  let i = 0;
  const len = body.length;
  let comments = [];

  function skipWs() {
    while (i < len && /\s/.test(body[i])) i++;
  }

  while (i < len) {
    skipWs();
    if (i >= len) break;

    // line comment
    if (body[i] === "/" && body[i + 1] === "/") {
      const end = body.indexOf("\n", i);
      const line = body.slice(i, end === -1 ? len : end);
      comments.push(line);
      i = end === -1 ? len : end + 1;
      continue;
    }

    // block comment — skip (rare in these files)
    if (body[i] === "/" && body[i + 1] === "*") {
      const end = body.indexOf("*/", i + 2);
      i = end === -1 ? len : end + 2;
      continue;
    }

    // Field start: identifier
    const idMatch = body.slice(i).match(/^(\w+)/);
    if (!idMatch) {
      i++;
      continue;
    }
    const fieldName = idMatch[1];
    i += fieldName.length;
    skipWs();

    // Embedded field: TypeName `tags` (no separate type token)
    if (body[i] === "`") {
      const tagEnd = body.indexOf("`", i + 1);
      const tags = body.slice(i + 1, tagEnd === -1 ? len : tagEnd);
      i = tagEnd === -1 ? len : tagEnd + 1;
      fields.push({
        name: fieldName,
        typeName: fieldName,
        tags,
        comments: comments.slice(),
      });
      comments = [];
      continue;
    }

    // Anonymous: Name struct {
    if (body.slice(i).startsWith("struct")) {
      i += "struct".length;
      skipWs();
      if (body[i] !== "{") {
        comments = [];
        continue;
      }
      const close = findMatchingBrace(body, i);
      if (close < 0) break;
      const anonBody = body.slice(i + 1, close);
      i = close + 1;
      skipWs();
      let tags = "";
      if (body[i] === "`") {
        const tagEnd = body.indexOf("`", i + 1);
        tags = body.slice(i + 1, tagEnd === -1 ? len : tagEnd);
        i = tagEnd === -1 ? len : tagEnd + 1;
      }
      fields.push({
        name: fieldName,
        typeName: "struct",
        tags,
        comments: comments.slice(),
        anonymousBody: anonBody,
      });
      comments = [];
      continue;
    }

    // Type (may include [], *, package.Type)
    const typeMatch = body.slice(i).match(/^(\*?\[\]?[\w.]+|\*?[\w.]+)/);
    if (!typeMatch) {
      comments = [];
      continue;
    }
    const typeName = typeMatch[1];
    i += typeName.length;
    skipWs();

    let tags = "";
    if (body[i] === "`") {
      const tagEnd = body.indexOf("`", i + 1);
      tags = body.slice(i + 1, tagEnd === -1 ? len : tagEnd);
      i = tagEnd === -1 ? len : tagEnd + 1;
    }

    fields.push({
      name: fieldName,
      typeName,
      tags,
      comments: comments.slice(),
    });
    comments = [];
  }

  return fields;
}

function parsePackageVars(source) {
  const vars = new Map();
  // var ( Name = value )
  const blockRe = /var\s*\(([^)]*)\)/gs;
  let bm;
  while ((bm = blockRe.exec(source)) !== null) {
    const block = bm[1];
    const lineRe = /(\w+)\s*=\s*([^\n]+)/g;
    let lm;
    while ((lm = lineRe.exec(block)) !== null) {
      vars.set(lm[1], lm[2].replace(/\s*\/\/.*$/, "").trim());
    }
  }
  // single var Name = value
  const singleRe = /^var\s+(\w+)\s*=\s*(.+)$/gm;
  let sm;
  while ((sm = singleRe.exec(source)) !== null) {
    vars.set(sm[1], sm[2].replace(/\s*\/\/.*$/, "").trim());
  }
  return vars;
}

function formatLiteralValue(expr, packageVars) {
  const e = expr.trim();

  if (e === "true" || e === "false") {
    return { ok: true, value: e };
  }
  if (/^-?\d+(\.\d+)?$/.test(e)) {
    return { ok: true, value: e };
  }
  if (e.startsWith('"') && e.endsWith('"')) {
    try {
      return { ok: true, value: JSON.parse(e) };
    } catch {
      return { ok: true, value: e.slice(1, -1) };
    }
  }

  // []int{1, 2, 3}
  const slice = e.match(/^\[\]int\{([^}]*)\}$/);
  if (slice) {
    const nums = slice[1]
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
    return { ok: true, value: `[${nums.join(", ")}]` };
  }

  // N*time.Second / N * time.Second
  const dur = e.match(/^(\d+)\s*\*\s*time\.(Second|Minute|Hour|Millisecond)$/);
  if (dur) {
    const n = dur[1];
    const unit = { Second: "s", Minute: "m", Hour: "h", Millisecond: "ms" }[dur[2]];
    return { ok: true, value: `${n}${unit}` };
  }

  // package var reference
  if (/^[A-Z]\w*$/.test(e) && packageVars.has(e)) {
    return formatLiteralValue(packageVars.get(e), packageVars);
  }

  return { ok: false, raw: e };
}

function findStringEnd(source, start, quote) {
  let i = start + 1;
  while (i < source.length) {
    const ch = source[i];
    if (quote === '"' && ch === "\\") {
      i += 2;
      continue;
    }
    if (ch === quote) return i;
    i++;
  }
  return -1;
}

function findValueEnd(source, start) {
  let depth = 0;
  let i = start;
  while (i < source.length) {
    const ch = source[i];
    if (ch === '"' || ch === "`") {
      const end = findStringEnd(source, i, ch);
      if (end === -1) return -1;
      i = end + 1;
      continue;
    }
    if (ch === "(") {
      depth++;
      i++;
      continue;
    }
    if (ch === ")") {
      if (depth === 0) return i;
      depth--;
      i++;
      continue;
    }
    i++;
  }
  return -1;
}

function parseSetDefaults(source, packageVars) {
  const defaults = new Map();
  const unresolved = [];
  const marker = "viper.SetDefault(";
  let i = 0;
  while (i < source.length) {
    const idx = source.indexOf(marker, i);
    if (idx === -1) break;
    let j = idx + marker.length;
    while (j < source.length && /\s/.test(source[j])) j++;
    if (source[j] !== '"') {
      i = j;
      continue;
    }
    const keyEnd = findStringEnd(source, j, '"');
    if (keyEnd === -1) {
      i = j;
      continue;
    }
    const key = source.slice(j + 1, keyEnd);
    let k = keyEnd + 1;
    while (k < source.length && /[\s,]/.test(source[k])) k++;
    const valueEnd = findValueEnd(source, k);
    if (valueEnd === -1) {
      i = k;
      continue;
    }
    const expr = source.slice(k, valueEnd).trim();
    const resolved = formatLiteralValue(expr, packageVars);
    if (resolved.ok) {
      defaults.set(key, String(resolved.value));
    } else {
      defaults.set(key, null);
      unresolved.push({ key, expression: resolved.raw || expr });
    }
    i = valueEnd + 1;
  }
  return { defaults, unresolved };
}

function displayType(typeName) {
  if (!typeName) return "";
  if (typeName === "logLevel") return "uint8";
  if (typeName === "ProverMode") return "string";
  if (typeName.startsWith("*")) return typeName.slice(1);
  return typeName;
}

function allowedRequiredCell(validate) {
  const parts = [];
  if (validate.required) parts.push("required");
  if (validate.oneof && validate.oneof.length) {
    parts.push(validate.oneof.join("|"));
  } else {
    for (const c of validate.constraints) {
      if (
        /^(gte|gt|lte|lt|semver|eth_addr|power_of_2)=/.test(c) ||
        /^(gte|gt|lte|lt|semver|eth_addr|power_of_2)$/.test(c)
      ) {
        parts.push(c);
      }
    }
  }
  return parts.join("; ");
}

function sectionForKey(key) {
  for (const s of SECTION_META) {
    if (s.noteOnly) continue;
    if (s.match(key)) return s;
  }
  return SECTION_META[0];
}

function walkFields({ fields, prefix, typeName, structs, keys, excluded, missingDescriptions }) {
  for (const field of fields) {
    const hard = HARD_EXCLUSIONS.find((e) => e.typeName === typeName && e.field === field.name);
    if (hard) {
      excluded.push({
        goField: `${typeName}.${field.name}`,
        reason: hard.reason,
      });
      continue;
    }

    const ms = parseMapstructure(field.tags);
    if (ms.skip) {
      excluded.push({
        goField: `${typeName}.${field.name}`,
        reason: 'mapstructure:"-"',
      });
      continue;
    }

    // Squash embed: flatten nested type into current prefix
    if (ms.squash || (ms.present && ms.name === "" && field.tags.includes("squash"))) {
      const nested = structs.get(field.typeName) || structs.get(field.name);
      if (!nested) {
        excluded.push({
          goField: `${typeName}.${field.name}`,
          reason: `squash target ${field.typeName} not found`,
        });
        continue;
      }
      const nestedFields = parseStructFields(nested);
      walkFields({
        fields: nestedFields,
        prefix,
        typeName: field.typeName,
        structs,
        keys,
        excluded,
        missingDescriptions,
      });
      continue;
    }

    const keyPart = ms.present && ms.name ? ms.name : field.name.toLowerCase();
    const key = prefix ? `${prefix}.${keyPart}` : keyPart;

    // Anonymous nested struct
    if (field.anonymousBody != null) {
      const nestedFields = parseStructFields(field.anonymousBody);
      walkFields({
        fields: nestedFields,
        prefix: key,
        typeName: `${typeName}.${field.name}`,
        structs,
        keys,
        excluded,
        missingDescriptions,
      });
      continue;
    }

    // Named nested struct type (recurse into known structs)
    if (structs.has(field.typeName) && !field.typeName.startsWith("[]")) {
      // TracesLimits: note-only — do not emit module rows
      if (field.typeName === "TracesLimits") {
        excluded.push({
          goField: "TracesLimits.ToLargeMode",
          reason: "runtime-only",
        });
        excluded.push({
          goField: "TracesLimits.ScalingFactor",
          reason: "runtime-only",
        });
        excluded.push({
          goField: "TracesLimits.Modules",
          reason: "traces_limits documented as note only (no module row dump)",
        });
        continue;
      }
      const nestedFields = parseStructFields(structs.get(field.typeName));
      walkFields({
        fields: nestedFields,
        prefix: key,
        typeName: field.typeName,
        structs,
        keys,
        excluded,
        missingDescriptions,
      });
      continue;
    }

    const description = cleanDescription(field.comments);
    const validate = parseValidate(field.tags);
    const entry = {
      key,
      goField: `${typeName}.${field.name}`,
      type: displayType(field.typeName),
      description,
      required: validate.required,
      oneof: validate.oneof,
      allowedRequired: allowedRequiredCell(validate),
      section: sectionForKey(key).id,
    };
    keys.push(entry);
    if (!description) {
      missingDescriptions.push({ key, goField: entry.goField });
    }
  }
}

/**
 * Extract documentable TOML keys from prover/config Go sources.
 * @param {string} configDir absolute path to prover/config
 */
function extractFromConfigDir(configDir) {
  const configGo = fs.readFileSync(path.join(configDir, "config.go"), "utf8");
  const defaultGo = fs.readFileSync(path.join(configDir, "config_default.go"), "utf8");
  // traces_limit.go informs the note; types.go may add aliases (read for completeness)
  fs.readFileSync(path.join(configDir, "traces_limit.go"), "utf8");
  if (fs.existsSync(path.join(configDir, "types.go"))) {
    fs.readFileSync(path.join(configDir, "types.go"), "utf8");
  }

  const structs = extractStructBodies(configGo);
  // ModuleLimit lives in traces_limit.go — load for completeness but unused for rows
  const tracesGo = fs.readFileSync(path.join(configDir, "traces_limit.go"), "utf8");
  for (const [name, body] of extractStructBodies(tracesGo)) {
    if (!structs.has(name)) structs.set(name, body);
  }

  const packageVars = parsePackageVars(defaultGo);
  const { defaults, unresolved } = parseSetDefaults(defaultGo, packageVars);

  const keys = [];
  const excluded = [];
  const missingDescriptions = [];

  const configBody = structs.get("Config");
  if (!configBody) {
    throw new Error("type Config struct not found in config.go");
  }

  walkFields({
    fields: parseStructFields(configBody),
    prefix: "",
    typeName: "Config",
    structs,
    keys,
    excluded,
    missingDescriptions,
  });

  // Attach defaults
  const unresolvedDefaults = [...unresolved];
  for (const k of keys) {
    if (defaults.has(k.key)) {
      const v = defaults.get(k.key);
      k.default = v;
      k.defaultResolved = v != null;
    } else {
      k.default = null;
      k.defaultResolved = false;
    }
  }

  // Sort keys for stable output
  keys.sort((a, b) => a.key.localeCompare(b.key));

  const perSection = {};
  for (const s of SECTION_META) {
    if (s.noteOnly) {
      perSection[s.id] = { title: s.title, keyCount: 0, noteOnly: true };
      continue;
    }
    const sectionKeys = keys.filter((k) => k.section === s.id);
    perSection[s.id] = { title: s.title, keyCount: sectionKeys.length, noteOnly: false };
  }

  const report = {
    excluded,
    missingDescriptions,
    unresolvedDefaults,
    tracesLimitsTreatment: "note-only (no module row dump)",
    envOnlyExcluded: excluded.filter((e) => /env-only/i.test(e.reason)),
  };

  const manifest = {
    generatedFrom: "prover/config (static Go parse)",
    note: "Public-safe TOML config keys. Defaults from config_default.go only; never from config-*.toml.",
    counts: {
      total: keys.length,
      sections: SECTION_META.length,
      rendered: keys.length,
      excluded: excluded.length,
    },
    perSection,
    sections: SECTION_META.map((s) => ({
      id: s.id,
      title: s.title,
      noteOnly: Boolean(s.noteOnly),
      partial: `${s.id}.mdx`,
    })),
    keys,
  };

  return { manifest, report, tracesLimitsNote: TRACES_LIMITS_NOTE, SECTION_META };
}

function extract(monorepoRoot) {
  const configDir = path.join(monorepoRoot, "prover", "config");
  return extractFromConfigDir(configDir);
}

module.exports = {
  SECTION_META,
  TRACES_LIMITS_NOTE,
  HARD_EXCLUSIONS,
  extract,
  extractFromConfigDir,
  cleanDescription,
  isDevNoiseComment,
  parseMapstructure,
  parseValidate,
  parseStructFields,
  extractStructBodies,
  parseSetDefaults,
  formatLiteralValue,
  sectionForKey,
};
