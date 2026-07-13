const { test } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");

const {
  resolveValueExpr,
  collectConstants,
  parseClassSource,
  discoverPlugins,
  buildManifest,
  renderPartials,
  renderStarterWrapper,
  checkCompleteness,
  isNeutralPartial,
  resolveMonorepoRoot,
  build,
  escapeCell,
  evalArithmetic,
  renderPluginPartial,
  PLUGIN_FLAG_PREFIX,
} = require("./lib");
const { MONOREPO_ROOT, TOOL_ROOT, MANIFEST_PATH, GENERATED_DIR, WRAPPER_TEMPLATE_PATH } = require("./paths");

const HAVE_SOURCE = fs.existsSync(path.join(MONOREPO_ROOT, "linea-besu/plugins"));
const skip = !HAVE_SOURCE && "linea-monorepo plugins not found";

test("escapeCell escapes MDX-sensitive characters", () => {
  assert.equal(escapeCell("a {b} c"), "a \\{b\\} c");
  assert.equal(escapeCell("use `code`"), "use &#96;code&#96;");
  assert.equal(escapeCell("a|b"), "a\\|b");
  assert.equal(escapeCell("a <b> c"), "a &lt;b&gt; c");
  assert.equal(escapeCell("path\\to"), "path\\\\to");
  assert.equal(escapeCell("${DEFAULT-VALUE}"), "$\\{DEFAULT-VALUE\\}");
});

test("evalArithmetic evaluates allowlisted expressions without Function()", () => {
  assert.equal(evalArithmetic("1024 * 1024 * 16"), 16777216);
  assert.equal(evalArithmetic("(1 + 2) * 3"), 9);
  assert.equal(evalArithmetic("10 / 2 - 1"), 4);
  assert.equal(evalArithmetic("alert(1)"), null);
  assert.equal(evalArithmetic("1; process.exit(1)"), null);
});

test("resolveValueExpr handles literals, constants, BigDecimal, arrays, Set.of", () => {
  const constants = collectConstants(`
    public static final long DEFAULT_A = 1_000_000_000;
    public static final long DEFAULT_B = 1024 * 1024 * 16L;
    public static final BigDecimal DEFAULT_C = BigDecimal.valueOf(0.5);
    public static final BigDecimal DEFAULT_D = BigDecimal.ONE;
    public static final double[] DEFAULT_E = { 0.1, 0.3, 1.0 };
    private static final Set<URL> DEFAULT_F = Set.of();
    public static final String NAME = "--plugin-linea-foo";
  `);
  assert.equal(resolveValueExpr("true", constants).display, "true");
  assert.equal(resolveValueExpr("DEFAULT_A", constants).display, "1000000000");
  assert.equal(resolveValueExpr("DEFAULT_B", constants).display, "16777216");
  assert.equal(resolveValueExpr("DEFAULT_C", constants).display, "0.5");
  assert.equal(resolveValueExpr("DEFAULT_D", constants).display, "1");
  assert.equal(resolveValueExpr("DEFAULT_E", constants).display, "[0.1, 0.3, 1.0]");
  assert.equal(resolveValueExpr("DEFAULT_F", constants).display, "[]");
  assert.equal(resolveValueExpr("NAME", constants).display, "--plugin-linea-foo");
});

test("resolveValueExpr never fabricates unresolvable method-call defaults", () => {
  const constants = collectConstants(`
    public static final String D = String.format("%s=%s", Foo.BAR.name(), Baz.now());
  `);
  assert.equal(resolveValueExpr("D", constants).resolved, false);
  assert.equal(resolveValueExpr("null", constants).resolved, false);
  assert.equal(resolveValueExpr("Address.ZERO", constants).resolved, false);
});

test("parseClassSource resolves names, concatenated descriptions and ${DEFAULT-VALUE}", () => {
  const source = `
    package net.consensys.linea.config;
    public class LineaSampleCliOptions {
      public static final String CONFIG_KEY = "sample-config";
      public static final String FOO = "--plugin-linea-foo";
      public static final int DEFAULT_FOO = 1024;
      @CommandLine.Option(
          names = {FOO},
          hidden = true,
          paramLabel = "<INTEGER>",
          description = "A foo value (default: \${DEFAULT-VALUE})")
      private int foo = DEFAULT_FOO;

      @Option(
          names = {"--plugin-linea-bar"},
          paramLabel = "<STRING>",
          description = "First part. " + "Second part.")
      private String bar = null;
    }
  `;
  const parsed = parseClassSource(source, "LineaSampleCliOptions.java");
  assert.equal(parsed.configKey, "sample-config");
  assert.equal(parsed.options.length, 2);

  const foo = parsed.options[0];
  assert.deepEqual(foo.names, ["--plugin-linea-foo"]);
  assert.equal(foo.hidden, true);
  assert.equal(foo.default, "1024");
  assert.equal(foo.description, "A foo value (default: 1024)");
  assert.equal(foo.type, "INTEGER");

  const bar = parsed.options[1];
  assert.deepEqual(bar.names, ["--plugin-linea-bar"]);
  assert.equal(bar.hidden, false);
  assert.equal(bar.default, null);
  assert.equal(bar.description, "First part. Second part.");
});

test("resolveMonorepoRoot defaults to this checkout", () => {
  assert.equal(resolveMonorepoRoot({}), MONOREPO_ROOT);
});

test("discovers sequencer + tracer + state-recovery plugins", { skip }, () => {
  const { plugins } = discoverPlugins(MONOREPO_ROOT);
  const byKey = Object.fromEntries(plugins.map((p) => [p.key, p]));
  assert.ok(byKey.sequencer && byKey.sequencer.hasOptions, "sequencer has options");
  assert.ok(byKey.tracer && byKey.tracer.hasOptions, "tracer has options");
  assert.ok(byKey["state-recovery"], "state-recovery is present");
  assert.equal(byKey["state-recovery"].hasOptions, false, "state-recovery has no options");
  assert.ok(!byKey["sequencer-interfaces"], "sequencer-interfaces is skipped");
});

test("forced-transactions group is excluded entirely", { skip }, () => {
  const discovery = discoverPlugins(MONOREPO_ROOT);
  const allClasses = discovery.plugins.flatMap((p) => p.classes.map((c) => c.className));
  assert.ok(!allClasses.includes("LineaForcedTransactionCliOptions"));
  assert.ok(
    discovery.excluded.some((g) => g.className === "LineaForcedTransactionCliOptions"),
    "forced-tx recorded as excluded",
  );
  const manifest = buildManifest(discovery);
  assert.ok(manifest.options.every((o) => o.configKey !== "forced-transaction-config"));
});

test("only plugin-specific (--plugin-*) options are included, each tagged by plugin", { skip }, async () => {
  const result = await build({
    monorepoPath: MONOREPO_ROOT,
    toolRoot: TOOL_ROOT,
  });
  for (const o of result.manifest.options) {
    assert.ok(o.names[0].startsWith(PLUGIN_FLAG_PREFIX), `${o.names[0]} is plugin-specific`);
    assert.ok(o.plugin, `${o.names[0]} carries a plugin field`);
  }
  const plugins = new Set(result.manifest.options.map((o) => o.plugin));
  assert.ok(plugins.has("sequencer") && plugins.has("tracer"), "options span >=2 plugins");
});

test("rendered row count equals in-scope option count (63 = 51 + 12)", { skip }, async () => {
  const result = await build({
    monorepoPath: MONOREPO_ROOT,
    toolRoot: TOOL_ROOT,
  });
  assert.equal(result.rowCount, result.manifest.counts.rendered);
  assert.equal(result.manifest.counts.total, 63);
  const byPlugin = Object.fromEntries(result.manifest.perPlugin.map((p) => [p.plugin, p.total]));
  assert.equal(byPlugin.sequencer, 51);
  assert.equal(byPlugin.tracer, 12);
  assert.equal(result.manifest.counts.standard + result.manifest.counts.advanced, result.manifest.counts.rendered);
  assert.ok(result.manifest.counts.advanced > 0);
  const sum = result.manifest.perPlugin.reduce((n, p) => n + p.total, 0);
  assert.equal(sum, result.manifest.counts.total);
});

test("partials are neutral; wrapper marks Advanced / forced-tx / state-recovery", { skip }, async () => {
  const result = await build({
    monorepoPath: MONOREPO_ROOT,
    toolRoot: TOOL_ROOT,
  });
  for (const p of result.partials) {
    assert.ok(isNeutralPartial(p.markdown), `${p.relPath} must be neutral`);
    assert.ok(p.markdown.includes("## "), `${p.relPath} has a plugin heading`);
    assert.ok(p.markdown.includes("### "), `${p.relPath} has a group heading`);
    assert.ok(p.markdown.includes("| Option"), `${p.relPath} has a table`);
    assert.ok(p.markdown.includes(" — "), `${p.relPath} uses unique plugin-prefixed group headings`);
  }
  assert.equal(result.partials.length, 2, "one partial per plugin with options");
  assert.ok(result.partials.some((p) => p.relPath === "sequencer.mdx"));
  assert.ok(result.partials.some((p) => p.relPath === "tracer.mdx"));
  assert.ok(result.wrapperMarkdown.includes(":::note Advanced options"));
  assert.ok(result.wrapperMarkdown.includes(":::note Forced transactions excluded"));
  assert.ok(result.wrapperMarkdown.includes("No plugin-specific CLI options"));
  assert.ok(!result.partials.some((p) => p.markdown.includes("--plugin-linea-forced-tx-status-cache-size")));
  const hidden = result.manifest.options.find((o) => o.hidden);
  assert.ok(hidden);
  assert.ok(result.partials.some((p) => p.markdown.includes(hidden.names[0])));
  assert.ok(result.partials.some((p) => p.markdown.includes("Advanced")));
  // Duplicate bare ### RPC headings must not appear (breaks Docusaurus TOC).
  const allHeadings = result.partials.flatMap((p) => [...p.markdown.matchAll(/^### (.+)$/gm)].map((m) => m[1]));
  assert.equal(new Set(allHeadings).size, allHeadings.length, `duplicate headings: ${allHeadings.join(", ")}`);
});

test("spot-check known defaults across plugins", { skip }, async () => {
  const result = await build({
    monorepoPath: MONOREPO_ROOT,
    toolRoot: TOOL_ROOT,
  });
  const byName = Object.fromEntries(result.manifest.options.map((o) => [o.names[0], o]));
  assert.equal(byName["--plugin-linea-variable-gas-cost-wei"].default, "10000000000");
  assert.equal(byName["--plugin-linea-tracer-readiness-server-port"].default, "8548");
});

test("completeness check passes for starter wrapper", { skip }, async () => {
  const result = await build({
    monorepoPath: MONOREPO_ROOT,
    toolRoot: TOOL_ROOT,
  });
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

test("generator is idempotent", { skip }, async () => {
  const a = await build({ monorepoPath: MONOREPO_ROOT, toolRoot: TOOL_ROOT });
  const b = await build({ monorepoPath: MONOREPO_ROOT, toolRoot: TOOL_ROOT });
  assert.equal(a.manifestJson, b.manifestJson);
  assert.equal(a.reportJson, b.reportJson);
  assert.equal(a.wrapperMarkdown, b.wrapperMarkdown);
  assert.equal(a.partials.length, b.partials.length);
  for (let i = 0; i < a.partials.length; i++) {
    assert.equal(a.partials[i].relPath, b.partials[i].relPath);
    assert.equal(a.partials[i].markdown, b.partials[i].markdown);
  }
});

test("output is public-safe: no real endpoints/keys/addresses in defaults", { skip }, async () => {
  const result = await build({
    monorepoPath: MONOREPO_ROOT,
    toolRoot: TOOL_ROOT,
  });
  for (const o of result.manifest.options) {
    if (o.default == null) continue;
    assert.doesNotMatch(o.default, /^https?:\/\//i, `${o.names[0]} default looks like a URL`);
    assert.doesNotMatch(o.default, /0x[a-fA-F0-9]{40}/, `${o.names[0]} default looks like an address`);
    assert.doesNotMatch(o.default, /^[A-Za-z0-9_-]{32,}$/, `${o.names[0]} default looks like a secret`);
  }
});

test("committed output matches freshly generated output (drift)", { skip }, async () => {
  const result = await build({
    monorepoPath: MONOREPO_ROOT,
    toolRoot: TOOL_ROOT,
  });
  if (fs.existsSync(MANIFEST_PATH)) {
    assert.equal(fs.readFileSync(MANIFEST_PATH, "utf8"), result.manifestJson);
  }
  for (const part of result.partials) {
    const file = path.join(GENERATED_DIR, part.relPath);
    if (!fs.existsSync(file)) continue;
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

test("renderPartials produces one file per plugin with unique group headings", { skip }, () => {
  const discovery = discoverPlugins(MONOREPO_ROOT);
  const manifest = buildManifest(discovery);
  const { partials, rowCount } = renderPartials(discovery.plugins);
  assert.equal(rowCount, manifest.counts.rendered);
  assert.equal(partials.length, 2);
  assert.ok(partials.some((p) => p.relPath === "sequencer.mdx"));
  assert.ok(partials.some((p) => p.relPath === "tracer.mdx"));
  assert.ok(partials.some((p) => p.markdown.includes("### Sequencer — RPC")));
  assert.ok(partials.some((p) => p.markdown.includes("### Tracer — RPC")));
  const wrapper = renderStarterWrapper(manifest, discovery.plugins, partials);
  assert.match(wrapper, /import\s+Sequencer\s+from\s+'\.\/_generated\/sequencer\.mdx'/);
  assert.match(wrapper, /import\s+Tracer\s+from\s+'\.\/_generated\/tracer\.mdx'/);
});

test("generated partials sanitize MDX-sensitive chars from @Option text", () => {
  const source = `
    package net.consensys.linea.config;
    public class LineaEvilCliOptions {
      public static final String CONFIG_KEY = "evil{key}";
      @CommandLine.Option(
          names = {"--plugin-linea-evil"},
          paramLabel = "<STRING>",
          description = "Uses {expr} and \`ticks\` and | pipes")
      private String evil = "default{x}";
    }
  `;
  const parsed = parseClassSource(source, "LineaEvilCliOptions.java");
  const plugin = {
    key: "evil",
    title: "Evil",
    hasOptions: true,
    classes: [parsed],
  };
  const { markdown } = renderPluginPartial(plugin);

  // No raw braces outside escaped form, and no raw backticks in cells.
  assert.match(markdown, /\\\{expr\\\}/);
  assert.match(markdown, /&#96;ticks&#96;/);
  assert.match(markdown, /\\\|/);
  assert.match(markdown, /evil\\\{key\\\}/);
  assert.match(markdown, /default\\\{x\\\}/);
  assert.doesNotMatch(markdown, /(?<!\\)\{expr\}/);
  assert.doesNotMatch(markdown, /(?<!\\)\{key\}/);
  // Inline-code backticks around escaped names/defaults are fine; raw option text backticks are not.
  assert.doesNotMatch(markdown, /Uses .*`ticks`/);
});
