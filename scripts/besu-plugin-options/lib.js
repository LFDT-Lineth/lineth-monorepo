/**
 * Linea-Besu plugin options generator (library).
 *
 * Statically parses the Linea-Besu plugin `*CliOptions.java` classes from this
 * monorepo and produces:
 *   - a JSON manifest of every in-scope plugin option (tagged by plugin), and
 *   - one neutral MDX partial per plugin under `_generated/`, plus an
 *     optional starter wrapper template that imports those partials.
 *
 * Only plugin-specific options (flags starting with `--plugin-`) are documented.
 * No monorepo compilation: the Java sources are parsed as text. Pure functions
 * only; all file I/O lives in generate.js / check.js.
 */

const fs = require("node:fs");
const path = require("node:path");

const prettier = require("prettier");

/**
 * Known plugin source roots (relative to the monorepo root), in display order.
 * Classes within each root are discovered dynamically, so new option classes
 * are picked up automatically. `tracer` lives outside `linea-besu/plugins`, so
 * it is listed explicitly here.
 */
const PLUGIN_SOURCES = [
  {
    key: "sequencer",
    title: "Sequencer",
    root: "linea-besu/plugins/linea-sequencer",
  },
  { key: "tracer", title: "Tracer", root: "tracer/arithmetization" },
  {
    key: "state-recovery",
    title: "State recovery",
    root: "linea-besu/plugins/state-recovery",
  },
];

/**
 * Directory whose subdirectories are auto-enumerated as additional plugins, so a
 * brand-new plugin module dropped here is picked up without code changes.
 */
const AUTO_PLUGIN_DIR = "linea-besu/plugins";

/** Plugin module directories that are not runtime plugins (no CLI options). */
const SKIP_PLUGIN_DIRS = new Set(["sequencer-interfaces"]);

/** Only flags with this prefix are in scope (plugin-specific options). */
const PLUGIN_FLAG_PREFIX = "--plugin-";

/** Class file excluded entirely from the manifest and page (unreleased feature). */
const EXCLUDED_CLASS_FILES = new Set(["LineaForcedTransactionCliOptions.java"]);

/** Acronyms to keep upper-cased in generated section titles. */
const TITLE_ACRONYMS = {
  Rpc: "RPC",
  Tx: "Tx",
  Tls: "TLS",
  Url: "URL",
  L1: "L1",
  L2: "L2",
};

// ---------------------------------------------------------------------------
// Low-level Java text helpers
// ---------------------------------------------------------------------------

/** Remove comments while preserving string/char literals and newlines. */
function stripComments(src) {
  let out = "";
  let state = "code";
  for (let i = 0; i < src.length; i++) {
    const c = src[i];
    const next = src[i + 1];
    if (state === "code") {
      if (c === '"') {
        out += c;
        state = "string";
      } else if (c === "'") {
        out += c;
        state = "char";
      } else if (c === "/" && next === "/") {
        state = "line";
        i++;
      } else if (c === "/" && next === "*") {
        state = "block";
        i++;
      } else {
        out += c;
      }
    } else if (state === "string") {
      out += c;
      if (c === "\\") out += src[++i] ?? "";
      else if (c === '"') state = "code";
    } else if (state === "char") {
      out += c;
      if (c === "\\") out += src[++i] ?? "";
      else if (c === "'") state = "code";
    } else if (state === "line") {
      if (c === "\n") {
        out += c;
        state = "code";
      }
    } else if (state === "block") {
      if (c === "\n") out += c;
      if (c === "*" && next === "/") {
        state = "code";
        i++;
      }
    }
  }
  return out;
}

/** Index of the `)` matching the `(` at `open`, respecting string literals. */
function matchParen(src, open) {
  let depth = 0;
  let state = "code";
  for (let i = open; i < src.length; i++) {
    const c = src[i];
    if (state === "code") {
      if (c === '"') state = "string";
      else if (c === "'") state = "char";
      else if (c === "(") depth++;
      else if (c === ")") {
        depth--;
        if (depth === 0) return i;
      }
    } else if (state === "string") {
      if (c === "\\") i++;
      else if (c === '"') state = "code";
    } else if (state === "char") {
      if (c === "\\") i++;
      else if (c === "'") state = "code";
    }
  }
  return -1;
}

/** Split a string on a top-level separator char, respecting brackets/strings. */
function splitTopLevel(s, sep) {
  const parts = [];
  let depth = 0;
  let cur = "";
  let state = "code";
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (state === "code") {
      if (c === '"') {
        state = "string";
        cur += c;
      } else if (c === "'") {
        state = "char";
        cur += c;
      } else if (c === "(" || c === "{" || c === "[") {
        depth++;
        cur += c;
      } else if (c === ")" || c === "}" || c === "]") {
        depth--;
        cur += c;
      } else if (c === sep && depth === 0) {
        parts.push(cur);
        cur = "";
      } else {
        cur += c;
      }
    } else if (state === "string") {
      cur += c;
      if (c === "\\") cur += s[++i] ?? "";
      else if (c === '"') state = "code";
    } else if (state === "char") {
      cur += c;
      if (c === "\\") cur += s[++i] ?? "";
      else if (c === "'") state = "code";
    }
  }
  if (cur.trim() !== "") parts.push(cur);
  return parts;
}

/** Unescape a Java double-quoted string literal (surrounding quotes optional). */
function parseJavaString(literal) {
  let s = literal.trim();
  if (s.startsWith('"') && s.endsWith('"')) s = s.slice(1, -1);
  return s.replace(/\\(u[0-9a-fA-F]{4}|.)/g, (m, esc) => {
    if (esc[0] === "u") return String.fromCharCode(parseInt(esc.slice(1), 16));
    const map = { n: "\n", t: "\t", r: "\r", '"': '"', "\\": "\\", "'": "'" };
    return map[esc] ?? esc;
  });
}

// ---------------------------------------------------------------------------
// Value resolution (constants, defaults, ${DEFAULT-VALUE})
// ---------------------------------------------------------------------------

/** Collect `static final <type> NAME = <expr>;` declarations from a class body. */
function collectConstants(src) {
  const constants = {};
  const re = /(?:public|private|protected)?\s*static\s+final\s+([\w.<>[\]]+)\s+([A-Za-z_]\w*)\s*=\s*([\s\S]*?);/g;
  let m;
  while ((m = re.exec(src)) !== null) {
    constants[m[2]] = { type: m[1], raw: m[3].trim() };
  }
  return constants;
}

/** Resolve a Java numeric literal/arithmetic expression to a display string. */
function resolveNumeric(raw) {
  const cleaned = raw.replace(/_/g, "").trim();

  // A single literal is preserved verbatim (minus the type suffix) so decimals
  // like `1.0` are not collapsed to `1`.
  const literal = cleaned.match(/^([-+]?\d+(?:\.\d+)?(?:[eE][-+]?\d+)?)[lLfFdD]?$/);
  if (literal) return { resolved: true, display: literal[1] };

  // Otherwise only evaluate genuine arithmetic (digits and operators only).
  const arithmetic = cleaned.replace(/([0-9])[lLfFdD]\b/g, "$1");
  if (!/^[\d.\s*+\-/()]+$/.test(arithmetic)) return { resolved: false };
  try {
    const value = Function('"use strict";return (' + arithmetic + ")")();
    if (typeof value !== "number" || !Number.isFinite(value)) {
      return { resolved: false };
    }
    return { resolved: true, display: String(value) };
  } catch {
    return { resolved: false };
  }
}

/**
 * Resolve a Java value expression to a display string.
 * Returns { resolved, display, isNull }.
 */
function resolveValueExpr(raw, constants, seen = new Set()) {
  const expr = raw.trim();
  if (expr === "" || expr === "null") return { resolved: false, isNull: expr === "null" };
  if (expr === "true" || expr === "false") return { resolved: true, display: expr };

  if (expr.includes('"')) return resolveStringConcat(expr, constants, seen);

  let m;
  if ((m = expr.match(/^BigDecimal\.(ONE|ZERO|TEN)$/))) {
    return {
      resolved: true,
      display: { ONE: "1", ZERO: "0", TEN: "10" }[m[1]],
    };
  }
  if ((m = expr.match(/^BigDecimal\.valueOf\(([\s\S]+)\)$/))) {
    return resolveValueExpr(m[1], constants, seen);
  }
  if ((m = expr.match(/^new\s+BigDecimal\(\s*"?([\s\S]+?)"?\s*\)$/))) {
    return resolveValueExpr(m[1], constants, seen);
  }
  if (/^Set\.of\(\s*\)$/.test(expr)) return { resolved: true, display: "[]" };
  if (/^List\.of\(\s*\)$/.test(expr)) return { resolved: true, display: "[]" };

  if ((m = expr.match(/^\{([\s\S]*)\}$/))) {
    const items = splitTopLevel(m[1], ",")
      .map((x) => x.trim())
      .filter(Boolean);
    const resolvedItems = items.map((it) => resolveValueExpr(it, constants, seen));
    if (resolvedItems.some((r) => !r.resolved)) return { resolved: false };
    return {
      resolved: true,
      display: "[" + resolvedItems.map((r) => r.display).join(", ") + "]",
    };
  }

  if (/^[A-Za-z_]\w*$/.test(expr)) {
    if (seen.has(expr)) return { resolved: false };
    const c = constants[expr];
    if (!c) return { resolved: false };
    return resolveValueExpr(c.raw, constants, new Set(seen).add(expr));
  }

  // Numeric literal / arithmetic (may legitimately contain "."). resolveNumeric
  // rejects anything with letters, so qualified names and method calls
  // (e.g. Class.CONST, String.format(...)) fall through to unresolved.
  return resolveNumeric(expr);
}

/**
 * Resolve a `"a" + CONST + "b"` style string concatenation. Only string
 * literals and bare constant identifiers are resolvable; method calls and
 * qualified references (e.g. `String.format(...)`) are treated as unresolved so
 * we never invent text.
 */
function resolveStringConcat(expr, constants, seen) {
  const terms = splitTopLevel(expr, "+")
    .map((t) => t.trim())
    .filter(Boolean);
  let out = "";
  for (const term of terms) {
    if (term.startsWith('"') && term.endsWith('"')) {
      out += parseJavaString(term);
    } else if (/^[A-Za-z_]\w*$/.test(term)) {
      const r = resolveValueExpr(term, constants, seen);
      if (!r.resolved) return { resolved: false };
      out += r.display;
    } else {
      return { resolved: false };
    }
  }
  return { resolved: true, display: out };
}

// ---------------------------------------------------------------------------
// Option parsing
// ---------------------------------------------------------------------------

/** Parse the attributes inside an `@Option(...)` annotation body. */
function parseAnnotationAttributes(body) {
  const attrs = {};
  for (const piece of splitTopLevel(body, ",")) {
    const m = piece.match(/^\s*([A-Za-z_]\w*)\s*=\s*([\s\S]+)$/);
    if (m) attrs[m[1]] = m[2].trim();
  }
  return attrs;
}

/** Map a Java field type / paramLabel into a display "Type". */
function displayType(paramLabel, javaType) {
  if (paramLabel) return paramLabel.replace(/^<|>$/g, "");
  return javaType;
}

/** Substitute resolved tokens into a description for human-readable rendering. */
function renderDescription(rawDescription, resolvedDefault, flags) {
  let text = rawDescription;
  if (text.includes("${DEFAULT-VALUE}")) {
    if (resolvedDefault != null) {
      text = text.split("${DEFAULT-VALUE}").join(resolvedDefault);
    } else {
      flags.unresolvedDefaultToken = true;
    }
  }
  if (text.includes("${COMPLETION-CANDIDATES}")) {
    flags.completionCandidatesToken = true;
  }
  return text;
}

/** Build a readable section title from a class name. */
function titleFromClassName(className) {
  let base = className.replace(/^Linea/, "").replace(/CliOptions$/, "");
  const words = base.match(/[A-Z][a-z0-9]*/g) || [base];
  const titled = words.map((w, i) => {
    if (TITLE_ACRONYMS[w]) return TITLE_ACRONYMS[w];
    return i === 0 ? w : w.toLowerCase();
  });
  return titled.join(" ").trim();
}

/** Parse a single `*CliOptions.java` source into a group record. */
function parseClassSource(rawSource, fileName) {
  const src = stripComments(rawSource);
  const className = fileName.replace(/\.java$/, "");
  const constants = collectConstants(src);

  const configKeyConst = constants["CONFIG_KEY"];
  let configKey = null;
  if (configKeyConst) {
    const r = resolveValueExpr(configKeyConst.raw, constants);
    if (r.resolved) configKey = r.display;
  }

  const options = [];
  const annotationRe = /@(?:CommandLine\.)?Option\s*\(/g;
  let m;
  while ((m = annotationRe.exec(src)) !== null) {
    const openParen = m.index + m[0].length - 1;
    const closeParen = matchParen(src, openParen);
    if (closeParen === -1) continue;
    const body = src.slice(openParen + 1, closeParen);
    const attrs = parseAnnotationAttributes(body);

    // The field declaration follows the annotation's closing paren.
    const after = src.slice(closeParen + 1);
    const fieldMatch = after.match(
      /^\s*(?:public|private|protected)?\s*(?:final\s+)?([\w.<>[\]]+)\s+([A-Za-z_]\w*)\s*(?:=\s*([\s\S]*?))?;/,
    );
    if (!fieldMatch) continue;
    const javaType = fieldMatch[1];
    const fieldInitializer = fieldMatch[3] ? fieldMatch[3].trim() : null;

    const line = src.slice(0, m.index).split("\n").length;

    // Names
    const namesRaw = attrs.names || "";
    const nameTokens = splitTopLevel(namesRaw.replace(/^\{|\}$/g, ""), ",")
      .map((t) => t.trim())
      .filter(Boolean);
    const names = nameTokens.map((tok) => {
      const r = resolveValueExpr(tok, constants);
      return r.resolved ? r.display : tok;
    });

    // hidden
    const hidden = /^true$/.test((attrs.hidden || "").trim());

    // Default: annotation defaultValue wins, else field initializer.
    const flags = {};
    let resolvedDefault = null;
    let defaultResolved = false;
    if (attrs.defaultValue != null) {
      const r = resolveValueExpr(attrs.defaultValue, constants);
      if (r.resolved) {
        resolvedDefault = r.display;
        defaultResolved = true;
      } else {
        flags.unresolvedDefault = attrs.defaultValue;
      }
    } else if (fieldInitializer != null) {
      const r = resolveValueExpr(fieldInitializer, constants);
      if (r.resolved) {
        resolvedDefault = r.display;
        defaultResolved = true;
      } else if (!r.isNull) {
        flags.unresolvedDefault = fieldInitializer;
      }
    }

    // Description
    const descriptionRaw = attrs.description ? resolveStringValue(attrs.description, constants) : "";
    const description = renderDescription(descriptionRaw, resolvedDefault, flags);

    const paramLabel = attrs.paramLabel ? resolveStringValue(attrs.paramLabel, constants) : null;

    options.push({
      group: configKey,
      configKey,
      sourceFile: fileName,
      sourceLine: line,
      names,
      description,
      descriptionRaw,
      default: resolvedDefault,
      defaultResolved,
      type: displayType(paramLabel, javaType),
      paramLabel,
      javaType,
      hidden,
      flags,
    });
  }

  return {
    className,
    title: titleFromClassName(className),
    configKey,
    options,
  };
}

/** Resolve a string-valued annotation attribute (literal or concatenation). */
function resolveStringValue(raw, constants) {
  const r = resolveValueExpr(raw, constants, new Set());
  if (r.resolved) return r.display;
  // Fall back to stripping quotes from the first literal so we never invent text.
  return raw.includes('"') ? parseJavaString(raw) : "";
}

// ---------------------------------------------------------------------------
// Plugin discovery + model
// ---------------------------------------------------------------------------

/** Recursively list files under a directory, skipping build/test output. */
function walkFiles(dir) {
  const out = [];
  let entries;
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return out;
  }
  for (const entry of entries) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (["build", ".gradle", "node_modules", "bin", "out"].includes(entry.name)) {
        continue;
      }
      out.push(...walkFiles(full));
    } else {
      out.push(full);
    }
  }
  return out;
}

/** Prettify an auto-discovered plugin directory name into a display title. */
function titleFromPluginKey(key) {
  const base = key
    .replace(/^linea-/, "")
    .replace(/-/g, " ")
    .trim();
  return base.charAt(0).toUpperCase() + base.slice(1);
}

/** Resolve the ordered list of plugin sources, including auto-discovered ones. */
function resolvePluginSources(monorepoRoot) {
  const sources = PLUGIN_SOURCES.map((s) => ({ ...s, auto: false }));
  const covered = new Set(sources.map((s) => s.root));

  const autoDirAbs = path.join(monorepoRoot, AUTO_PLUGIN_DIR);
  let autoNames = [];
  try {
    autoNames = fs
      .readdirSync(autoDirAbs, { withFileTypes: true })
      .filter((d) => d.isDirectory())
      .map((d) => d.name)
      .sort();
  } catch {
    autoNames = [];
  }
  for (const name of autoNames) {
    if (SKIP_PLUGIN_DIRS.has(name)) continue;
    const root = `${AUTO_PLUGIN_DIR}/${name}`;
    if (covered.has(root)) continue;
    covered.add(root);
    sources.push({
      key: name,
      title: titleFromPluginKey(name),
      root,
      auto: true,
    });
  }
  return sources;
}

/**
 * Discover every in-scope plugin, its option classes, and its `--plugin-*`
 * options. Returns { plugins, excluded } where each plugin lists its classes
 * (only classes that contribute at least one in-scope option) or carries an
 * empty `classes` array when the plugin exposes no plugin-specific CLI options.
 */
function discoverPlugins(monorepoRoot) {
  const sources = resolvePluginSources(monorepoRoot);
  const plugins = [];
  const excluded = [];

  for (const source of sources) {
    const rootAbs = path.join(monorepoRoot, source.root);
    const exists = fs.existsSync(rootAbs);
    const files = exists
      ? walkFiles(rootAbs)
          .filter((f) => f.endsWith("CliOptions.java"))
          .filter((f) => !f.replace(/\\/g, "/").includes("/src/test/"))
          .sort()
      : [];

    const classes = [];
    for (const file of files) {
      const fileName = path.basename(file);
      const sourceText = fs.readFileSync(file, "utf8");
      const parsed = parseClassSource(sourceText, fileName);
      const inScope = parsed.options.filter((o) => o.names[0] && o.names[0].startsWith(PLUGIN_FLAG_PREFIX));
      if (inScope.length === 0) continue; // interface / non-plugin class
      if (EXCLUDED_CLASS_FILES.has(fileName)) {
        excluded.push({
          plugin: source.key,
          pluginTitle: source.title,
          className: parsed.className,
          configKey: parsed.configKey,
          optionCount: inScope.length,
        });
        continue;
      }
      for (const o of inScope) {
        o.plugin = source.key;
        o.pluginTitle = source.title;
        o.className = parsed.className;
        o.classTitle = parsed.title;
      }
      classes.push({
        className: parsed.className,
        title: parsed.title,
        configKey: parsed.configKey,
        options: inScope,
      });
    }
    classes.sort((a, b) => a.className.localeCompare(b.className));
    plugins.push({
      key: source.key,
      title: source.title,
      root: source.root,
      exists,
      auto: source.auto,
      hasOptions: classes.length > 0,
      classes,
    });
  }
  return { plugins, excluded };
}

/** Flatten plugins to a single ordered option list (manifest-ready). */
function flattenOptions(plugins) {
  const options = [];
  for (const p of plugins) {
    for (const c of p.classes) {
      for (const o of c.options) {
        const flat = { ...o };
        delete flat.flags;
        options.push(flat);
      }
    }
  }
  return options;
}

/** Per-plugin standard/advanced/total breakdown. */
function pluginBreakdown(plugins) {
  return plugins.map((p) => {
    const total = p.classes.reduce((n, c) => n + c.options.length, 0);
    const advanced = p.classes.reduce((n, c) => n + c.options.filter((o) => o.hidden).length, 0);
    return {
      plugin: p.key,
      title: p.title,
      standard: total - advanced,
      advanced,
      total,
      classes: p.classes.length,
      hasOptions: p.hasOptions,
    };
  });
}

/** Build the manifest object from discovered plugins. */
function buildManifest({ plugins, excluded }) {
  const options = flattenOptions(plugins);
  const hidden = options.filter((o) => o.hidden).length;
  const groups = plugins.reduce((n, p) => n + p.classes.length, 0);
  return {
    generatedFrom: "linea-monorepo (Linea-Besu plugins)",
    note: "Generated by scripts/besu-plugin-options. Do not edit by hand.",
    hiddenTreatment: "Hidden options are included and marked Advanced (real operator flags not surfaced in CLI help).",
    scope: "Plugin-specific options only (flags starting with --plugin-).",
    counts: {
      plugins: plugins.length,
      groups,
      total: options.length,
      standard: options.length - hidden,
      advanced: hidden,
      hidden,
      rendered: options.length,
      excludedGroups: excluded.length,
      excludedOptions: excluded.reduce((n, g) => n + g.optionCount, 0),
    },
    perPlugin: pluginBreakdown(plugins),
    excludedGroups: excluded.map((g) => ({
      plugin: g.plugin,
      className: g.className,
      configKey: g.configKey,
      optionCount: g.optionCount,
      reason: "Unreleased feature (forced transactions). TODO: include once shipped.",
    })),
    plugins: plugins.map((p) => ({
      key: p.key,
      title: p.title,
      root: p.root,
      hasOptions: p.hasOptions,
      classes: p.classes.map((c) => ({
        className: c.className,
        title: c.title,
        configKey: c.configKey,
        optionCount: c.options.length,
      })),
    })),
    options,
  };
}

/** Build a report of anything flagged during parsing. */
function buildReport({ plugins, excluded }) {
  const missingDescriptions = [];
  const unresolvedDefaults = [];
  const unresolvedTokens = [];
  let advanced = 0;
  const emptyPlugins = [];

  for (const p of plugins) {
    if (!p.hasOptions) {
      emptyPlugins.push({
        plugin: p.key,
        note: "No plugin-specific CLI options found.",
      });
      continue;
    }
    for (const c of p.classes) {
      for (const o of c.options) {
        const flag = o.names[0] || "(unknown)";
        if (o.hidden) advanced++;
        if (!o.descriptionRaw) {
          missingDescriptions.push({
            plugin: p.key,
            option: flag,
            sourceFile: o.sourceFile,
          });
        }
        if (o.flags && o.flags.unresolvedDefault) {
          unresolvedDefaults.push({
            plugin: p.key,
            option: flag,
            sourceFile: o.sourceFile,
            expression: o.flags.unresolvedDefault,
          });
        }
        if (o.flags && o.flags.unresolvedDefaultToken) {
          unresolvedTokens.push({
            plugin: p.key,
            option: flag,
            token: "${DEFAULT-VALUE}",
          });
        }
        if (o.flags && o.flags.completionCandidatesToken) {
          unresolvedTokens.push({
            plugin: p.key,
            option: flag,
            token: "${COMPLETION-CANDIDATES}",
          });
        }
      }
    }
  }
  return {
    hiddenTreatment: "Hidden options are included and marked Advanced (real operator flags not surfaced in CLI help).",
    excludedGroups: excluded.map((g) => ({
      plugin: g.plugin,
      className: g.className,
      optionCount: g.optionCount,
      reason: "Unreleased feature (forced transactions).",
    })),
    pluginsWithoutOptions: emptyPlugins,
    advancedOptionCount: advanced,
    missingDescriptions,
    unresolvedDefaults,
    unresolvedTokens,
  };
}

// ---------------------------------------------------------------------------
// Markdown / MDX rendering (partials + starter wrapper)
// ---------------------------------------------------------------------------

/** Escape a value for safe inclusion in a Markdown/MDX table cell. */
function escapeCell(text) {
  if (text == null) return "";
  let out = String(text).replace(/\r?\n/g, " ").replace(/\s+/g, " ").trim();
  // Protect MDX from `{...}` expressions and `<...>` JSX by wrapping leftover
  // template tokens in inline code, then escape backslashes and table pipes.
  out = out.replace(/\$\{[^}]+\}/g, (t) => "`" + t + "`");
  out = out.replace(/</g, "&lt;").replace(/>/g, "&gt;");
  out = out.replace(/\\/g, "\\\\");
  out = out.replace(/\|/g, "\\|");
  return out;
}

/** Render the option name(s) as inline code. */
function renderNames(names) {
  if (!names.length) return "";
  return names.map((n) => "`" + n + "`").join("<br/>");
}

/** Render the default value cell. */
function renderDefault(option) {
  if (option.default == null || option.default === "") return "";
  return "`" + escapeCell(option.default) + "`";
}

/** Stable filename slug for a group (config key, else class name). */
function groupSlug(cls) {
  if (cls.configKey) return cls.configKey;
  return cls.className
    .replace(/^Linea/, "")
    .replace(/CliOptions$/, "")
    .replace(/([a-z0-9])([A-Z])/g, "$1-$2")
    .toLowerCase();
}

/** Relative path of a plugin partial under `_generated/` (posix). */
function partialRelPath(pluginKey) {
  return `${pluginKey}.mdx`;
}

/** Capitalized React component name for a default MDX import. */
function partialComponentName(pluginKey) {
  return pluginKey
    .split(/[^A-Za-z0-9]+/)
    .filter(Boolean)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join("");
}

/** Unique section heading so Docusaurus TOC anchors do not collide across plugins. */
function sectionHeading(pluginTitle, groupTitle) {
  return `${pluginTitle} — ${groupTitle}`;
}

/**
 * Render one option-group section (heading + config key + table) into `lines`.
 * Returns the number of option rows written.
 */
function renderGroupSection(lines, pluginTitle, cls) {
  lines.push(`### ${sectionHeading(pluginTitle, cls.title)}`);
  lines.push("");
  if (cls.configKey) {
    lines.push(`Config-file key: \`${cls.configKey}\``);
    lines.push("");
  }
  lines.push("| Option | Description | Default | Type | Visibility |");
  lines.push("| --- | --- | --- | --- | --- |");
  let rowCount = 0;
  for (const o of cls.options) {
    const row = [
      renderNames(o.names),
      escapeCell(o.description),
      renderDefault(o),
      escapeCell(o.type),
      o.hidden ? "Advanced" : "Standard",
    ];
    lines.push("| " + row.join(" | ") + " |");
    rowCount++;
  }
  lines.push("");
  return rowCount;
}

/**
 * Render one neutral MDX partial for a whole plugin (all option groups).
 * No front matter, no imports, no custom React components.
 */
function renderPluginPartial(plugin) {
  const lines = [];
  lines.push(`## ${plugin.title}`);
  lines.push("");
  let rowCount = 0;
  for (const cls of plugin.classes) {
    rowCount += renderGroupSection(lines, plugin.title, cls);
  }
  return {
    markdown: lines.join("\n").replace(/\s+$/, "") + "\n",
    rowCount,
  };
}

/**
 * Render all generated partials. Returns { partials, rowCount } where partials
 * is one entry per plugin that has options:
 * { plugin, relPath, componentName, title, markdown, rowCount }.
 */
function renderPartials(plugins) {
  const partials = [];
  let rowCount = 0;
  for (const p of plugins) {
    if (!p.hasOptions) continue;
    const { markdown, rowCount: n } = renderPluginPartial(p);
    rowCount += n;
    partials.push({
      plugin: p.key,
      relPath: partialRelPath(p.key),
      componentName: partialComponentName(p.key),
      title: p.title,
      markdown,
      rowCount: n,
    });
  }
  return { partials, rowCount };
}

/**
 * One-time starter wrapper for doc.linea. Human-owned after seeding; never
 * overwritten by normal generate runs.
 */
function renderStarterWrapper(manifest, plugins, partials) {
  const partByPlugin = new Map(partials.map((p) => [p.plugin, p]));

  const lines = [];
  lines.push("---");
  lines.push("title: Linea-Besu plugin options");
  lines.push("slug: /stack/reference/linea-besu-plugin-options");
  lines.push("description: Auto-generated reference of Linea-Besu plugin CLI options, grouped by plugin and feature.");
  lines.push("draft: false");
  lines.push("---");
  lines.push("");
  lines.push(
    "{/* Human-owned wrapper. Automation only updates `_generated/` partials. " +
      "Seeded once by scripts/besu-plugin-options; place new partial imports when plugins appear. */}",
  );
  lines.push("");

  for (const part of partials) {
    lines.push(`import ${part.componentName} from './_generated/${part.relPath}';`);
  }
  if (partials.length) lines.push("");

  lines.push(":::note Advanced options");
  lines.push("");
  lines.push(
    "Options marked **Advanced** are flagged `hidden` in the source: they are real operator flags but are not surfaced in the CLI help output. " +
      "They are included here intentionally so operators can discover and tune them.",
  );
  lines.push("");
  lines.push(":::");
  lines.push("");
  lines.push(":::note Forced transactions excluded");
  lines.push("");
  lines.push(
    "The `LineaForcedTransactionCliOptions` group is intentionally excluded because the forced-transactions feature is unreleased. " +
      "TODO: include this group once the feature ships.",
  );
  lines.push("");
  lines.push(":::");
  lines.push("");

  const c = manifest.counts;
  lines.push(
    `This reference documents **${c.total} plugin options** across **${c.plugins} plugins** and **${c.groups} option groups** ` +
      `(${c.standard} standard, ${c.advanced} advanced). Only plugin-specific options (flags starting with \`--plugin-\`) are listed. ` +
      "Defaults and descriptions are taken verbatim from the source.",
  );
  lines.push("");

  for (const p of plugins) {
    const part = partByPlugin.get(p.key);
    if (part) {
      lines.push(`<${part.componentName} />`);
      lines.push("");
      continue;
    }
    lines.push(`## ${p.title}`);
    lines.push("");
    lines.push("_No plugin-specific CLI options were found in this plugin._");
    lines.push("");
  }

  return lines.join("\n").replace(/\s+$/, "") + "\n";
}

/**
 * Completeness: every generated partial must be imported by the wrapper, and
 * every `_generated/` import in the wrapper must exist.
 */
function checkCompleteness(wrapperMarkdown, partialRelPaths) {
  const failures = [];
  const importRe = /import\s+([A-Za-z_][\w]*)\s+from\s+['"]\.\/_generated\/([^'"]+)['"]/g;
  const imported = new Map();
  let m;
  while ((m = importRe.exec(wrapperMarkdown)) !== null) {
    imported.set(m[2].replace(/\\/g, "/"), m[1]);
  }

  const expected = new Set(partialRelPaths.map((p) => p.replace(/\\/g, "/")));
  for (const rel of expected) {
    if (!imported.has(rel)) {
      failures.push(
        `generated partial _generated/${rel} is not imported by the wrapper (place the import and <Component />)`,
      );
    } else {
      const name = imported.get(rel);
      const usage = new RegExp(`<${name}\\s*/>`);
      if (!usage.test(wrapperMarkdown)) {
        failures.push(`wrapper imports ${name} from _generated/${rel} but never renders <${name} />`);
      }
    }
  }
  for (const rel of imported.keys()) {
    if (!expected.has(rel)) {
      failures.push(`wrapper imports _generated/${rel} but that partial was not generated`);
    }
  }
  return failures;
}

/** True if partial markdown is neutral (no front matter / imports / custom components). */
function isNeutralPartial(markdown) {
  if (markdown.trimStart().startsWith("---")) return false;
  if (/^import\s+/m.test(markdown)) return false;
  // Partials are markdown tables only — no custom JSX components.
  if (/<[A-Z][A-Za-z0-9]*/.test(markdown)) return false;
  return true;
}

// ---------------------------------------------------------------------------
// Top-level orchestration (pure: returns strings, no writes)
// ---------------------------------------------------------------------------

/** Resolve the monorepo root (defaults to this checkout). */
function resolveMonorepoRoot({ monorepoPath, monorepoRoot } = {}) {
  if (monorepoPath) return path.resolve(monorepoPath);
  if (process.env.LINEA_MONOREPO_PATH) return path.resolve(process.env.LINEA_MONOREPO_PATH);
  if (monorepoRoot) return path.resolve(monorepoRoot);
  // This tool lives at scripts/besu-plugin-options → monorepo is ../..
  return path.resolve(__dirname, "..", "..");
}

/** Format generated text with Prettier (canonical output). */
async function formatWith(text, ext, toolRoot) {
  const filepath = path.join(toolRoot, `besu-plugin-options-output.${ext}`);
  const config = (await prettier.resolveConfig(filepath)) || {};
  return prettier.format(text, { ...config, filepath });
}

/**
 * Produce Prettier-formatted manifest JSON, report, partials, and starter
 * wrapper strings. Output is canonical so generate + drift-check agree.
 */
async function build({ monorepoPath, monorepoRoot, toolRoot } = {}) {
  const root = toolRoot || __dirname;
  const resolvedMonorepo = resolveMonorepoRoot({ monorepoPath, monorepoRoot });
  const autoDir = path.join(resolvedMonorepo, AUTO_PLUGIN_DIR);
  if (!fs.existsSync(autoDir)) {
    throw new Error(
      `linea-monorepo plugins directory not found: ${autoDir}\n` +
        `Point the generator at a linea-monorepo checkout via --monorepo <path> or LINEA_MONOREPO_PATH.`,
    );
  }
  const discovery = discoverPlugins(resolvedMonorepo);
  const manifest = buildManifest(discovery);
  const report = buildReport(discovery);
  const { partials, rowCount } = renderPartials(discovery.plugins);

  if (rowCount !== manifest.counts.rendered) {
    throw new Error(
      `Count mismatch: rendered ${rowCount} rows but manifest counts ${manifest.counts.rendered} in-scope options.`,
    );
  }
  if (manifest.counts.standard + manifest.counts.advanced !== manifest.counts.rendered) {
    throw new Error(
      `Count mismatch: standard (${manifest.counts.standard}) + advanced (${manifest.counts.advanced}) != rendered (${manifest.counts.rendered}).`,
    );
  }

  const wrapperMarkdown = renderStarterWrapper(manifest, discovery.plugins, partials);

  const [manifestJson, reportJson, formattedWrapper, ...formattedPartials] = await Promise.all([
    formatWith(JSON.stringify(manifest, null, 2), "json", root),
    formatWith(JSON.stringify(report, null, 2), "json", root),
    formatWith(wrapperMarkdown, "mdx", root),
    ...partials.map((p) => formatWith(p.markdown, "mdx", root)),
  ]);

  const formattedPartialsList = partials.map((p, i) => ({
    ...p,
    markdown: formattedPartials[i],
  }));

  for (const p of formattedPartialsList) {
    if (!isNeutralPartial(p.markdown)) {
      throw new Error(`Partial _generated/${p.relPath} is not neutral (front matter, import, or custom JSX).`);
    }
  }

  return {
    monorepoRoot: resolvedMonorepo,
    manifestJson,
    reportJson,
    wrapperMarkdown: formattedWrapper,
    partials: formattedPartialsList,
    manifest,
    report,
    rowCount,
    plugins: discovery.plugins,
  };
}

module.exports = {
  PLUGIN_SOURCES,
  EXCLUDED_CLASS_FILES,
  PLUGIN_FLAG_PREFIX,
  stripComments,
  matchParen,
  splitTopLevel,
  parseJavaString,
  collectConstants,
  resolveValueExpr,
  parseClassSource,
  discoverPlugins,
  pluginBreakdown,
  buildManifest,
  buildReport,
  groupSlug,
  partialRelPath,
  partialComponentName,
  sectionHeading,
  renderGroupSection,
  renderPluginPartial,
  renderPartials,
  renderStarterWrapper,
  checkCompleteness,
  isNeutralPartial,
  resolveMonorepoRoot,
  build,
};
