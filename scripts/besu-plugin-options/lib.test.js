const { test } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const {
  escapeCell,
  stripEmbeddedDefault,
  renderDefault,
  renderType,
  build,
  pluginsForRender,
  renderPartials,
  renderStarterWrapper,
  renderPluginPartial,
  checkCompleteness,
  isNeutralPartial,
  PLUGIN_FLAG_PREFIX,
} = require("./lib");
const { MANIFEST_PATH, REPORT_PATH, GENERATED_DIR, WRAPPER_TEMPLATE_PATH } = require("./paths");

const HAVE_MANIFEST = fs.existsSync(MANIFEST_PATH) && fs.existsSync(REPORT_PATH);
const skip = !HAVE_MANIFEST && "Java-extracted manifest not found (run pnpm run generate)";

test("escapeCell escapes MDX-sensitive characters", () => {
  assert.equal(escapeCell("a {b} c"), "a \\{b\\} c");
  assert.equal(escapeCell("use `code`"), "use &#96;code&#96;");
  assert.equal(escapeCell("a|b"), "a\\|b");
  assert.equal(escapeCell("a <b> c"), "a &lt;b&gt; c");
  assert.equal(escapeCell("path\\to"), "path\\\\to");
  assert.equal(escapeCell("${DEFAULT-VALUE}"), "$\\{DEFAULT-VALUE\\}");
});

test("stripEmbeddedDefault removes picocli help default parentheticals", () => {
  assert.equal(
    stripEmbeddedDefault("Path to the toml file (default: moduleLimitFile.toml)"),
    "Path to the toml file",
  );
  assert.equal(stripEmbeddedDefault("Forward endpoints (default: [])"), "Forward endpoints");
  assert.equal(stripEmbeddedDefault("No default here"), "No default here");
});

test("renderType keeps angle brackets inside backticks", () => {
  assert.equal(renderType("SET<URL>"), "`SET<URL>`");
  assert.equal(renderType("BYTES_32"), "`BYTES_32`");
});

test("renderDefault uses em dash when unset", () => {
  assert.equal(renderDefault({ default: null }), "—");
  assert.equal(renderDefault({ default: "" }), "—");
  assert.equal(renderDefault({ default: "1000" }), "`1000`");
});

test("rendered row count equals in-scope option count (63 = 51 + 12)", { skip }, async () => {
  const result = await build({});
  assert.equal(result.rowCount, result.manifest.counts.rendered);
  assert.equal(result.manifest.counts.total, 63);
  const byPlugin = Object.fromEntries(result.manifest.perPlugin.map((p) => [p.plugin, p.total]));
  assert.equal(byPlugin.sequencer, 51);
  assert.equal(byPlugin.tracer, 12);
  assert.equal(result.manifest.counts.standard + result.manifest.counts.advanced, result.manifest.counts.rendered);
});

test("only plugin-specific (--plugin-*) options are included", { skip }, async () => {
  const result = await build({});
  for (const o of result.manifest.options) {
    assert.ok(o.names[0].startsWith(PLUGIN_FLAG_PREFIX), `${o.names[0]} is plugin-specific`);
    assert.ok(o.plugin, `${o.names[0]} carries a plugin field`);
  }
});

test("partials are neutral with unique plugin-prefixed headings", { skip }, async () => {
  const result = await build({});
  for (const p of result.partials) {
    assert.ok(isNeutralPartial(p.markdown), `${p.relPath} must be neutral`);
    assert.ok(p.markdown.includes("## "), `${p.relPath} has a plugin heading`);
    assert.ok(p.markdown.includes("### "), `${p.relPath} has a group heading`);
    assert.ok(p.markdown.includes("| Option"), `${p.relPath} has a table`);
    assert.ok(p.markdown.includes(" — "), `${p.relPath} uses unique plugin-prefixed group headings`);
    assert.equal(
      /\(default:/i.test(p.markdown),
      false,
      `${p.relPath} must not embed (default: …) when a Default column exists`,
    );
    assert.equal(/\|[^|\n]*&lt;[^|\n]*\|/.test(p.markdown), false, `${p.relPath} must not HTML-escape types`);
    assert.match(p.markdown, /\| —\s*\|/, `${p.relPath} uses em dash for unset defaults`);
  }
  assert.equal(result.partials.length, 2);
  assert.ok(result.partials.some((p) => p.relPath === "sequencer.mdx"));
  assert.ok(result.partials.some((p) => p.relPath === "tracer.mdx"));
  const allHeadings = result.partials.flatMap((p) => [...p.markdown.matchAll(/^### (.+)$/gm)].map((m) => m[1]));
  assert.equal(new Set(allHeadings).size, allHeadings.length, `duplicate headings: ${allHeadings.join(", ")}`);
});

test("spot-check known defaults across plugins", { skip }, async () => {
  const result = await build({});
  const byName = Object.fromEntries(result.manifest.options.map((o) => [o.names[0], o]));
  assert.equal(byName["--plugin-linea-variable-gas-cost-wei"].default, "10000000000");
  assert.equal(byName["--plugin-linea-tracer-readiness-server-port"].default, "8548");
  assert.equal(byName["--plugin-linea-l1l2-bridge-contract"].default, "0x0000000000000000000000000000000000000000");
  assert.equal(byName["--plugin-linea-l1l2-bridge-topic"].type, "BYTES_32");
  assert.match(byName["--plugin-linea-compressed-tx-cache-size"].description, /compressed transaction cache/i);
  assert.doesNotMatch(byName["--plugin-linea-compressed-tx-cache-size"].description, /Variable gas cost/i);
});

test("completeness check passes for starter wrapper", { skip }, async () => {
  const result = await build({});
  const failures = checkCompleteness(
    result.wrapperMarkdown,
    result.partials.map((p) => p.relPath),
  );
  assert.deepEqual(failures, []);
});

test("completeness fails when a partial is missing from the wrapper", () => {
  const failures = checkCompleteness(`import Sequencer from './_generated/sequencer.mdx';\n\n<Sequencer />\n`, [
    "sequencer.mdx",
    "tracer.mdx",
  ]);
  assert.ok(failures.some((f) => f.includes("tracer.mdx")));
});

test("completeness allows publish-only provenance import", () => {
  const failures = checkCompleteness(
    [
      "import Provenance from './_generated/provenance.mdx';",
      "import Sequencer from './_generated/sequencer.mdx';",
      "",
      "<Provenance />",
      "<Sequencer />",
      "",
    ].join("\n"),
    ["sequencer.mdx"],
  );
  assert.deepEqual(failures, []);
});

test("generator is idempotent", { skip }, async () => {
  const a = await build({});
  const b = await build({});
  assert.equal(a.manifestJson, b.manifestJson);
  assert.equal(a.reportJson, b.reportJson);
  assert.equal(a.wrapperMarkdown, b.wrapperMarkdown);
  assert.equal(a.partials.length, b.partials.length);
  for (let i = 0; i < a.partials.length; i++) {
    assert.equal(a.partials[i].markdown, b.partials[i].markdown);
  }
});

test("committed MDX matches freshly rendered output (drift)", { skip }, async () => {
  const result = await build({});
  for (const part of result.partials) {
    const file = path.join(GENERATED_DIR, part.relPath);
    assert.ok(fs.existsSync(file), part.relPath);
    assert.equal(fs.readFileSync(file, "utf8"), part.markdown, part.relPath);
  }
  if (fs.existsSync(WRAPPER_TEMPLATE_PATH)) {
    const failures = checkCompleteness(
      fs.readFileSync(WRAPPER_TEMPLATE_PATH, "utf8"),
      result.partials.map((p) => p.relPath),
    );
    assert.deepEqual(failures, []);
  }
});

test("renderPartials produces one file per plugin with unique group headings", { skip }, async () => {
  const result = await build({});
  const plugins = pluginsForRender(result.manifest);
  const { partials } = renderPartials(plugins);
  assert.ok(partials.some((p) => p.markdown.includes("### Sequencer — RPC")));
  assert.ok(partials.some((p) => p.markdown.includes("### Tracer — RPC")));
  const wrapper = renderStarterWrapper(result.manifest, plugins, partials);
  assert.match(wrapper, /import\s+Sequencer\s+from\s+'\.\/_generated\/sequencer\.mdx'/);
  assert.match(wrapper, /import\s+Tracer\s+from\s+'\.\/_generated\/tracer\.mdx'/);
});

test("generated partials sanitize MDX-sensitive chars from option text", async () => {
  const manifest = {
    counts: { total: 1, plugins: 1, groups: 1, standard: 1, advanced: 0, rendered: 1 },
    plugins: [
      {
        key: "evil",
        title: "Evil",
        hasOptions: true,
        classes: [{ className: "EvilCliOptions", title: "Evil", configKey: "evil{key}", optionCount: 1 }],
      },
    ],
    options: [
      {
        plugin: "evil",
        className: "EvilCliOptions",
        names: ["--plugin-linea-evil"],
        description: "Uses {expr} and `ticks` and | pipes",
        default: "default{x}",
        type: "STRING",
        hidden: false,
      },
    ],
  };
  const plugins = pluginsForRender(manifest);
  const { markdown } = renderPluginPartial(plugins[0]);
  assert.match(markdown, /\\\{expr\\\}/);
  assert.match(markdown, /&#96;ticks&#96;/);
  assert.match(markdown, /\\\|/);
  assert.match(markdown, /evil\\\{key\\\}/);
  assert.match(markdown, /default\\\{x\\\}/);
});

test("forced-transactions group is excluded in manifest", { skip }, async () => {
  const result = await build({});
  assert.ok(result.manifest.options.every((o) => o.configKey !== "forced-transaction-config"));
  assert.ok(result.manifest.excludedGroups.some((g) => g.className === "LineaForcedTransactionCliOptions"));
});
