/**
 * Linea prover config MDX renderer.
 * Extraction is done by parse-go.js; this file turns the extract into MDX partials + wrapper.
 */

const path = require("node:path");
const prettier = require("prettier");

const { extract, SECTION_META, TRACES_LIMITS_NOTE } = require("./parse-go");

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

function escapeInlineCode(text) {
  if (text == null) return "";
  let out = String(text).replace(/\r?\n/g, " ").replace(/\s+/g, " ").trim();
  out = out.replace(/\\/g, "\\\\");
  out = out.replace(/`/g, "&#96;");
  out = out.replace(/\{/g, "\\{").replace(/\}/g, "\\}");
  out = out.replace(/\|/g, "\\|");
  return out;
}

function renderDefault(value) {
  if (value == null || value === "") return "—";
  return "`" + escapeInlineCode(value) + "`";
}

function renderType(type) {
  if (type == null || type === "") return "";
  return "`" + escapeInlineCode(type) + "`";
}

function partialRelPath(sectionId) {
  return `${sectionId}.mdx`;
}

function partialComponentName(sectionId) {
  return sectionId
    .split(/[^A-Za-z0-9]+/)
    .filter(Boolean)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join("");
}

function renderSectionPartial(section, keys, tracesNote) {
  const lines = [];
  lines.push(`### ${section.title}`);
  lines.push("");

  if (section.noteOnly) {
    for (const para of tracesNote.split("\n")) {
      lines.push(para);
    }
    lines.push("");
    return { markdown: lines.join("\n").replace(/\s+$/, "") + "\n", rowCount: 0 };
  }

  lines.push("| Config key | Description | Default | Type | Allowed / required |");
  lines.push("| --- | --- | --- | --- | --- |");
  let rowCount = 0;
  for (const k of keys) {
    const row = [
      "`" + escapeInlineCode(k.key) + "`",
      escapeCell(k.description),
      renderDefault(k.default),
      renderType(k.type),
      escapeCell(k.allowedRequired || ""),
    ];
    lines.push("| " + row.join(" | ") + " |");
    rowCount++;
  }
  lines.push("");
  return { markdown: lines.join("\n").replace(/\s+$/, "") + "\n", rowCount };
}

function renderPartials(manifest, tracesNote) {
  const keysBySection = new Map();
  for (const k of manifest.keys) {
    if (!keysBySection.has(k.section)) keysBySection.set(k.section, []);
    keysBySection.get(k.section).push(k);
  }

  const partials = [];
  let rowCount = 0;
  for (const s of SECTION_META) {
    const sectionKeys = keysBySection.get(s.id) || [];
    // Sort within section by key
    sectionKeys.sort((a, b) => a.key.localeCompare(b.key));
    const { markdown, rowCount: n } = renderSectionPartial(s, sectionKeys, tracesNote);
    rowCount += n;
    partials.push({
      section: s.id,
      relPath: partialRelPath(s.id),
      componentName: partialComponentName(s.id),
      title: s.title,
      markdown,
      rowCount: n,
      noteOnly: Boolean(s.noteOnly),
    });
  }
  return { partials, rowCount };
}

function renderStarterWrapper(partials) {
  const lines = [];
  lines.push("---");
  lines.push("title: Linea prover configuration");
  lines.push("slug: /stack/reference/linea-prover-options");
  lines.push("description: Auto-generated reference of Linea prover TOML configuration keys, grouped by section.");
  lines.push("draft: false");
  lines.push("---");
  lines.push("");
  lines.push(
    "{/* Human-owned wrapper. Automation only updates `_generated/prover/` partials. " +
      "Seeded once by scripts/prover-options; place new partial imports when sections appear. */}",
  );
  lines.push("");

  lines.push("import Provenance from './_generated/prover/provenance.mdx';");
  for (const part of partials) {
    lines.push(`import ${part.componentName} from './_generated/prover/${part.relPath}';`);
  }
  lines.push("");

  lines.push(
    "This reference lists Linea prover TOML configuration keys, grouped by section. " +
      "Descriptions come from Go doc comments on the config structs; defaults come from " +
      "`config_default.go` (never from sample `config-*.toml` files).",
  );
  lines.push("");
  lines.push("<Provenance />");
  lines.push("");

  for (const part of partials) {
    lines.push(`<${part.componentName} />`);
    lines.push("");
  }

  return lines.join("\n").replace(/\s+$/, "") + "\n";
}

/**
 * Completeness: every generated partial imported + rendered; no dangling imports.
 * Publish-only provenance.mdx is allowed under _generated/prover/.
 */
function checkCompleteness(wrapperMarkdown, partialRelPaths) {
  const failures = [];
  const importRe = /import\s+([A-Za-z_][\w]*)\s+from\s+['"]\.\/_generated\/prover\/([^'"]+)['"]/g;
  const imported = new Map();
  let m;
  while ((m = importRe.exec(wrapperMarkdown)) !== null) {
    imported.set(m[2].replace(/\\/g, "/"), m[1]);
  }

  const publishOnlyPartials = new Set(["provenance.mdx"]);
  const expected = new Set(partialRelPaths.map((p) => p.replace(/\\/g, "/")));

  for (const rel of expected) {
    if (!imported.has(rel)) {
      failures.push(
        `generated partial _generated/prover/${rel} is not imported by the wrapper (place the import and <Component />)`,
      );
    } else {
      const name = imported.get(rel);
      const usage = new RegExp(`<${name}\\s*/>`);
      if (!usage.test(wrapperMarkdown)) {
        failures.push(`wrapper imports ${name} from _generated/prover/${rel} but never renders <${name} />`);
      }
    }
  }
  for (const rel of imported.keys()) {
    if (expected.has(rel) || publishOnlyPartials.has(rel)) {
      if (publishOnlyPartials.has(rel) && !expected.has(rel)) {
        const name = imported.get(rel);
        const usage = new RegExp(`<${name}\\s*/>`);
        if (!usage.test(wrapperMarkdown)) {
          failures.push(`wrapper imports ${name} from _generated/prover/${rel} but never renders <${name} />`);
        }
      }
      continue;
    }
    failures.push(`wrapper imports _generated/prover/${rel} but that partial was not generated`);
  }
  return failures;
}

function isNeutralPartial(markdown) {
  if (markdown.trimStart().startsWith("---")) return false;
  if (/^import\s+/m.test(markdown)) return false;
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
  const filepath = path.join(toolRoot, `prover-options-output.${ext}`);
  const config = (await prettier.resolveConfig(filepath)) || {};
  return prettier.format(text, { ...config, filepath });
}

/**
 * Extract from Go + render MDX.
 */
async function build({ monorepoRoot, toolRoot } = {}) {
  const root = toolRoot || __dirname;
  const mono = resolveMonorepoRoot({ monorepoRoot });
  const { manifest, report, tracesLimitsNote } = extract(mono);
  return buildFromExtract({ manifest, report, tracesLimitsNote, toolRoot: root });
}

async function buildFromExtract({ manifest, report, tracesLimitsNote, toolRoot } = {}) {
  const root = toolRoot || __dirname;
  const note = tracesLimitsNote || TRACES_LIMITS_NOTE;
  const { partials, rowCount } = renderPartials(manifest, note);

  if (rowCount !== manifest.counts.rendered) {
    throw new Error(
      `Count mismatch: rendered ${rowCount} rows but manifest counts ${manifest.counts.rendered} documentable keys.`,
    );
  }

  const wrapperMarkdown = renderStarterWrapper(partials);
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
      throw new Error(`Partial _generated/prover/${p.relPath} is not neutral (front matter, import, or custom JSX).`);
    }
  }

  return {
    manifestJson,
    reportJson,
    wrapperMarkdown: formattedWrapper,
    partials: formattedPartialsList,
    manifest,
    report,
    rowCount,
  };
}

module.exports = {
  escapeCell,
  escapeInlineCode,
  renderDefault,
  renderType,
  partialRelPath,
  partialComponentName,
  renderPartials,
  renderStarterWrapper,
  checkCompleteness,
  isNeutralPartial,
  resolveMonorepoRoot,
  build,
  buildFromExtract,
  SECTION_META,
};
