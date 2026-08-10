/**
 * Linea-Besu plugin options MDX renderer.
 *
 * Extraction is done by the Java Gradle module
 * `:linea-besu:plugins:besu-plugin-options-docgen` (reflection over @Option).
 * This file turns the ephemeral JSON manifest into MDX partials + wrapper.
 */

const fs = require("node:fs");
const path = require("node:path");
const prettier = require("prettier");

const PLUGIN_FLAG_PREFIX = "--plugin-";

/**
 * Escape a value for safe inclusion in a Markdown/MDX table cell or inline code.
 * Escapes `\`, backticks, `{`, `}`, `<`, `>`, and `|` so merged Java @Option text
 * cannot become MDX expressions or break table markup.
 */
function escapeCell(text) {
  if (text == null) return "";
  let out = String(text).replace(/\r?\n/g, " ").replace(/\s+/g, " ").trim();
  out = out.replace(/\\/g, "\\\\");
  out = out.replace(/`/g, "&#96;");
  out = out.replace(/\{/g, "\\{").replace(/\}/g, "\\}");
  out = out.replace(/</g, "&lt;").replace(/>/g, "&gt;");
  out = out.replace(/\|/g, "\\|");
  return out;
}

/**
 * Drop picocli-help "(default: …)" from descriptions — the table already has a Default column.
 */
function stripEmbeddedDefault(description) {
  if (description == null) return "";
  return String(description)
    .replace(/\s*\(default:\s*[^)]*\)/gi, "")
    .replace(/\s{2,}/g, " ")
    .trim();
}

function renderNames(names) {
  if (!names.length) return "";
  return names.map((n) => "`" + escapeCell(n) + "`").join("<br/>");
}

/**
 * Escape for inclusion inside inline code (backticks). Unlike escapeCell, does not
 * turn <> into HTML entities so types like SET<URL> stay readable.
 */
function escapeInlineCode(text) {
  if (text == null) return "";
  let out = String(text).replace(/\r?\n/g, " ").replace(/\s+/g, " ").trim();
  out = out.replace(/\\/g, "\\\\");
  out = out.replace(/`/g, "&#96;");
  out = out.replace(/\{/g, "\\{").replace(/\}/g, "\\}");
  out = out.replace(/\|/g, "\\|");
  return out;
}

function renderDefault(option) {
  if (option.default == null || option.default === "") return "—";
  return "`" + escapeInlineCode(option.default) + "`";
}

function renderType(type) {
  if (type == null || type === "") return "";
  return "`" + escapeInlineCode(type) + "`";
}

function partialRelPath(pluginKey) {
  return `${pluginKey}.mdx`;
}

function partialComponentName(pluginKey) {
  return pluginKey
    .split(/[^A-Za-z0-9]+/)
    .filter(Boolean)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join("");
}

function sectionHeading(pluginTitle, groupTitle) {
  return `${pluginTitle} — ${groupTitle}`;
}

function renderGroupSection(lines, pluginTitle, cls) {
  lines.push(`### ${sectionHeading(pluginTitle, cls.title)}`);
  lines.push("");
  if (cls.configKey) {
    lines.push(`Config-file key: \`${escapeCell(cls.configKey)}\``);
    lines.push("");
  }
  lines.push("| Option | Description | Default | Type | Visibility |");
  lines.push("| --- | --- | --- | --- | --- |");
  let rowCount = 0;
  for (const o of cls.options) {
    const row = [
      renderNames(o.names),
      escapeCell(stripEmbeddedDefault(o.description)),
      renderDefault(o),
      renderType(o.type),
      o.hidden ? "Advanced" : "Standard",
    ];
    lines.push("| " + row.join(" | ") + " |");
    rowCount++;
  }
  lines.push("");
  return rowCount;
}

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
 * Nest flat manifest.options back under plugin.classes for rendering.
 */
function pluginsForRender(manifest) {
  const optionsByClass = new Map();
  for (const o of manifest.options) {
    const key = `${o.plugin}::${o.className}`;
    if (!optionsByClass.has(key)) optionsByClass.set(key, []);
    optionsByClass.get(key).push(o);
  }

  return manifest.plugins.map((p) => ({
    key: p.key,
    title: p.title,
    root: p.root,
    hasOptions: p.hasOptions,
    classes: (p.classes || []).map((c) => ({
      className: c.className,
      title: c.title,
      configKey: c.configKey,
      options: optionsByClass.get(`${p.key}::${c.className}`) || [],
    })),
  }));
}

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
    "{/* Human-owned wrapper. Automation only updates `_generated/besu/` partials. " +
      "Seeded once by scripts/besu-plugin-options; place new partial imports when plugins appear. */}",
  );
  lines.push("");

  lines.push("import Provenance from './_generated/besu/provenance.mdx';");
  for (const part of partials) {
    lines.push(`import ${part.componentName} from './_generated/besu/${part.relPath}';`);
  }
  lines.push("");

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

  lines.push(
    "This reference lists plugin-specific options (flags starting with `--plugin-`), grouped by plugin and feature. " +
      "Descriptions come from the Java `@Option` sources; defaults are shown in the Default column " +
      "(picocli help text `(default: …)` is omitted there to avoid duplication).",
  );
  lines.push("");

  lines.push("<Provenance />");
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

function checkCompleteness(wrapperMarkdown, partialRelPaths) {
  const failures = [];
  const importRe = /import\s+([A-Za-z_][\w]*)\s+from\s+['"]\.\/_generated\/besu\/([^'"]+)['"]/g;
  const imported = new Map();
  let m;
  while ((m = importRe.exec(wrapperMarkdown)) !== null) {
    imported.set(m[2].replace(/\\/g, "/"), m[1]);
  }

  // Publish workflow stamps this; not produced by local/CI `pnpm run generate`.
  const publishOnlyPartials = new Set(["provenance.mdx"]);

  const expected = new Set(partialRelPaths.map((p) => p.replace(/\\/g, "/")));
  for (const rel of expected) {
    if (!imported.has(rel)) {
      failures.push(
        `generated partial _generated/besu/${rel} is not imported by the wrapper (place the import and <Component />)`,
      );
    } else {
      const name = imported.get(rel);
      const usage = new RegExp(`<${name}\\s*/>`);
      if (!usage.test(wrapperMarkdown)) {
        failures.push(`wrapper imports ${name} from _generated/besu/${rel} but never renders <${name} />`);
      }
    }
  }
  for (const rel of imported.keys()) {
    if (expected.has(rel) || publishOnlyPartials.has(rel)) {
      if (publishOnlyPartials.has(rel) && !expected.has(rel)) {
        const name = imported.get(rel);
        const usage = new RegExp(`<${name}\\s*/>`);
        if (!usage.test(wrapperMarkdown)) {
          failures.push(`wrapper imports ${name} from _generated/besu/${rel} but never renders <${name} />`);
        }
      }
      continue;
    }
    failures.push(`wrapper imports _generated/besu/${rel} but that partial was not generated`);
  }
  return failures;
}

function isNeutralPartial(markdown) {
  if (markdown.trimStart().startsWith("---")) return false;
  if (/^import\s+/m.test(markdown)) return false;
  // Types like SET<URL> live in inline code; ignore those when scanning for JSX.
  const withoutInlineCode = markdown.replace(/`[^`]*`/g, "");
  if (/<[A-Z][A-Za-z0-9]*/.test(withoutInlineCode)) return false;
  return true;
}

function resolveMonorepoRoot({ monorepoPath, monorepoRoot } = {}) {
  if (monorepoPath) return path.resolve(monorepoPath);
  if (process.env.LINEA_MONOREPO_PATH) return path.resolve(process.env.LINEA_MONOREPO_PATH);
  if (monorepoRoot) return path.resolve(monorepoRoot);
  return path.resolve(__dirname, "..", "..");
}

async function formatWith(text, ext, toolRoot) {
  const filepath = path.join(toolRoot, `besu-plugin-options-output.${ext}`);
  const config = (await prettier.resolveConfig(filepath)) || {};
  return prettier.format(text, { ...config, filepath });
}

function loadManifest(manifestPath) {
  if (!fs.existsSync(manifestPath)) {
    throw new Error(
      `Manifest not found at ${manifestPath}. Run ` +
        `./gradlew :linea-besu:plugins:besu-plugin-options-docgen:generateBesuPluginOptionsManifest first.`,
    );
  }
  return JSON.parse(fs.readFileSync(manifestPath, "utf8"));
}

function loadReport(reportPath) {
  if (!fs.existsSync(reportPath)) {
    throw new Error(`Report not found at ${reportPath}.`);
  }
  return JSON.parse(fs.readFileSync(reportPath, "utf8"));
}

/**
 * Render MDX from an already-extracted Java manifest (+ report).
 */
async function buildFromManifest({ manifest, report, toolRoot } = {}) {
  const root = toolRoot || __dirname;
  // Canonicalize descriptions so the Default column is the only place defaults appear
  // (picocli help text embeds "(default: …)" in @Option descriptions).
  const canonicalManifest = {
    ...manifest,
    options: (manifest.options || []).map((o) => ({
      ...o,
      description: stripEmbeddedDefault(o.description),
    })),
  };
  const plugins = pluginsForRender(canonicalManifest);
  const { partials, rowCount } = renderPartials(plugins);

  if (rowCount !== canonicalManifest.counts.rendered) {
    throw new Error(
      `Count mismatch: rendered ${rowCount} rows but manifest counts ${canonicalManifest.counts.rendered} in-scope options.`,
    );
  }

  const wrapperMarkdown = renderStarterWrapper(canonicalManifest, plugins, partials);
  const [manifestJson, reportJson, formattedWrapper, ...formattedPartials] = await Promise.all([
    formatWith(JSON.stringify(canonicalManifest, null, 2), "json", root),
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
      throw new Error(`Partial _generated/besu/${p.relPath} is not neutral (front matter, import, or custom JSX).`);
    }
  }

  return {
    manifestJson,
    reportJson,
    wrapperMarkdown: formattedWrapper,
    partials: formattedPartialsList,
    manifest: canonicalManifest,
    report,
    rowCount,
    plugins,
  };
}

async function build({ manifestPath, reportPath, toolRoot } = {}) {
  const root = toolRoot || __dirname;
  const { MANIFEST_PATH, REPORT_PATH } = require("./paths");
  const manifest = loadManifest(manifestPath || MANIFEST_PATH);
  const report = loadReport(reportPath || REPORT_PATH);
  return buildFromManifest({ manifest, report, toolRoot: root });
}

module.exports = {
  PLUGIN_FLAG_PREFIX,
  escapeCell,
  escapeInlineCode,
  stripEmbeddedDefault,
  renderDefault,
  renderType,
  partialRelPath,
  partialComponentName,
  sectionHeading,
  renderGroupSection,
  renderPluginPartial,
  renderPartials,
  renderStarterWrapper,
  pluginsForRender,
  checkCompleteness,
  isNeutralPartial,
  resolveMonorepoRoot,
  loadManifest,
  loadReport,
  buildFromManifest,
  build,
};
