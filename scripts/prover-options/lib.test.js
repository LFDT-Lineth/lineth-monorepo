const { test } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

const {
  escapeCell,
  isNeutralPartial,
  checkCompleteness,
  resolveMonorepoRoot,
  build,
  partialComponentName,
} = require("./lib");
const { cleanDescription, isDevNoiseComment, extract } = require("./parse-go");
const { TOOL_ROOT, MONOREPO_ROOT, OUTPUT_DIR, WRAPPER_TEMPLATE_PATH } = require("./paths");

test("escapeCell escapes MDX-sensitive characters", () => {
  assert.equal(escapeCell("a|b"), "a\\|b");
  assert.equal(escapeCell("a{b}"), "a\\{b\\}");
  assert.equal(escapeCell("<x>"), "&lt;x&gt;");
});

test("dev-noise comments are stripped from descriptions", () => {
  assert.equal(isDevNoiseComment("TODO @gbotrel fix this"), true);
  assert.equal(isDevNoiseComment("@gbotrel alone at line start"), true);
  assert.equal(isDevNoiseComment("not serialized"), true);
  assert.equal(isDevNoiseComment("for testing purposes only"), true);
  assert.equal(isDevNoiseComment("duplicate from Config.Layer2"), true);
  assert.equal(isDevNoiseComment("The delays at which we retry"), false);
  assert.equal(cleanDescription(["// TODO @gbotrel noise", "// Real description here"]), "Real description here");
  // Mid-line @gbotrel must not truncate useful text (AssetsDir two-line doc comment).
  assert.equal(
    cleanDescription([
      "// AssetsDir stores the root of the directory where the assets are stored (setup) or",
      "// accessed (prover). The file structure is described in TODO @gbotrel.",
    ]),
    "AssetsDir stores the root of the directory where the assets are stored (setup) or accessed (prover).",
  );
});

test("partialComponentName", () => {
  assert.equal(partialComponentName("data-availability"), "DataAvailability");
  assert.equal(partialComponentName("traces-limits"), "TracesLimits");
});

test("checkCompleteness fails when a partial is missing from the wrapper", () => {
  const failures = checkCompleteness("import Foo from './_generated/prover/foo.mdx';\n<Foo />\n", [
    "foo.mdx",
    "bar.mdx",
  ]);
  assert.ok(failures.some((f) => f.includes("bar.mdx")));
});

test("checkCompleteness allows publish-only provenance.mdx", () => {
  const wrapper =
    "import Provenance from './_generated/prover/provenance.mdx';\n" +
    "import Foo from './_generated/prover/foo.mdx';\n" +
    "<Provenance />\n<Foo />\n";
  const failures = checkCompleteness(wrapper, ["foo.mdx"]);
  assert.deepEqual(failures, []);
});

test("isNeutralPartial rejects front matter and imports", () => {
  assert.equal(isNeutralPartial("---\ntitle: x\n---\n"), false);
  assert.equal(isNeutralPartial("import X from './x.mdx';\n"), false);
  assert.equal(isNeutralPartial("### Hello\n\n| a | b |\n"), true);
});

test("check validates fresh temporary output without committed artifacts", () => {
  const backupDir = `${OUTPUT_DIR}.test-backup-${process.pid}`;
  const hadOutput = fs.existsSync(OUTPUT_DIR);
  if (hadOutput) fs.renameSync(OUTPUT_DIR, backupDir);

  try {
    const result = spawnSync("node", ["check.js"], {
      cwd: TOOL_ROOT,
      encoding: "utf8",
      env: process.env,
    });
    assert.equal(result.status, 0, result.stderr || result.stdout);
    assert.equal(fs.existsSync(OUTPUT_DIR), false, "check should not recreate the normal output directory");
  } finally {
    fs.rmSync(OUTPUT_DIR, { recursive: true, force: true });
    if (hadOutput) fs.renameSync(backupDir, OUTPUT_DIR);
  }
});

test("explicit --monorepo path wins over LINEA_MONOREPO_PATH", () => {
  const result = spawnSync("node", ["check.js", "--monorepo", MONOREPO_ROOT], {
    cwd: TOOL_ROOT,
    encoding: "utf8",
    env: {
      ...process.env,
      LINEA_MONOREPO_PATH: path.join(TOOL_ROOT, "missing-env-monorepo"),
    },
  });

  assert.equal(result.status, 0, result.stderr || result.stdout);
});

test("monorepo root resolution uses environment override before default fallback", () => {
  const previous = process.env.LINEA_MONOREPO_PATH;
  const environmentRoot = path.join(TOOL_ROOT, "environment-monorepo");
  const defaultRoot = path.join(TOOL_ROOT, "default-monorepo");

  try {
    process.env.LINEA_MONOREPO_PATH = environmentRoot;
    assert.equal(resolveMonorepoRoot({ monorepoRoot: defaultRoot }), path.resolve(environmentRoot));

    delete process.env.LINEA_MONOREPO_PATH;
    assert.equal(resolveMonorepoRoot({ monorepoRoot: defaultRoot }), path.resolve(defaultRoot));
  } finally {
    if (previous === undefined) {
      delete process.env.LINEA_MONOREPO_PATH;
    } else {
      process.env.LINEA_MONOREPO_PATH = previous;
    }
  }
});

test("extract discovers documentable keys and excludes env-only/-", () => {
  const { manifest, report } = extract(MONOREPO_ROOT);
  assert.ok(manifest.keys.length > 40, `expected many keys, got ${manifest.keys.length}`);
  const keySet = new Set(manifest.keys.map((k) => k.key));
  assert.equal(keySet.has("controller.localid"), false);
  assert.equal(keySet.has("layer2.message_service_contract"), true);
  assert.equal(keySet.has("layer2.msgsvccontract"), false);
  assert.ok(report.excluded.some((e) => /LocalID/.test(e.goField)));
  assert.ok(report.excluded.some((e) => /mapstructure:"-"/.test(e.reason)));
  assert.equal(manifest.perSection["traces-limits"].noteOnly, true);
});

test("rendered rows match documentable key count", async () => {
  const result = await build({ monorepoRoot: MONOREPO_ROOT, toolRoot: TOOL_ROOT });
  assert.equal(result.rowCount, result.manifest.counts.total);
  assert.equal(result.rowCount, result.manifest.keys.length);
});

test("spot-check known defaults and allowed values", () => {
  const { manifest } = extract(MONOREPO_ROOT);
  const byKey = new Map(manifest.keys.map((k) => [k.key, k]));

  assert.equal(byKey.get("controller.spot_instance_reclaim_time_seconds").default, "120");
  assert.equal(byKey.get("controller.termination_grace_period_seconds").default, "2700");
  assert.equal(byKey.get("data_availability.max_nb_batches").default, "100");
  assert.equal(byKey.get("data_availability.dict_nb_bytes").default, "65536");

  const profile = byKey.get("debug.performance_monitor.profile");
  assert.equal(profile.default, "prover-rounds");
  assert.deepEqual(profile.oneof, ["prover-steps", "prover-rounds", "all"]);

  const mode = byKey.get("execution.prover_mode");
  assert.deepEqual(mode.oneof, ["dev", "partial", "full", "proofless", "bench", "check-only", "limitless"]);

  // Unresolved cross-pkg default stays blank
  assert.equal(byKey.get("data_availability.max_uncompressed_nb_bytes").default, null);
});

test("partials are neutral and traces-limits is a note", async () => {
  const result = await build({ monorepoRoot: MONOREPO_ROOT, toolRoot: TOOL_ROOT });
  const partials = new Map(result.partials.map((partial) => [partial.relPath, partial.markdown]));
  assert.ok(partials.has("traces-limits.mdx"));
  for (const [relativePath, markdown] of partials) {
    assert.equal(isNeutralPartial(markdown), true, `${relativePath} should be neutral`);
  }
  const note = partials.get("traces-limits.mdx");
  assert.match(note, /prefix/i);
  assert.doesNotMatch(note, /\| Config key \|/);
});

test("wrapper completeness against fresh partials", async () => {
  assert.ok(fs.existsSync(WRAPPER_TEMPLATE_PATH));
  const wrapper = fs.readFileSync(WRAPPER_TEMPLATE_PATH, "utf8");
  const result = await build({ monorepoRoot: MONOREPO_ROOT, toolRoot: TOOL_ROOT });
  const partials = result.partials.map((partial) => partial.relPath);
  const failures = checkCompleteness(wrapper, partials);
  assert.deepEqual(failures, []);
});

test("output is public-safe: no config-*.toml secrets leaked", async () => {
  const configDir = path.join(MONOREPO_ROOT, "prover", "config");
  const tomlFiles = fs.readdirSync(configDir).filter((f) => f.startsWith("config-") && f.endsWith(".toml"));
  assert.ok(tomlFiles.length > 0);

  // Intentional defaults from config_default.go (paths, cmds) are allowed.
  const result = await build({ monorepoRoot: MONOREPO_ROOT, toolRoot: TOOL_ROOT });
  const { manifest } = result;
  const allowedDefaults = new Set(manifest.keys.map((k) => k.default).filter((d) => d != null && d !== ""));

  const leakedCandidates = new Set();
  for (const f of tomlFiles) {
    const text = fs.readFileSync(path.join(configDir, f), "utf8");
    for (const m of text.matchAll(/0x[a-fA-F0-9]{40}/g)) leakedCandidates.add(m[0]);
    for (const m of text.matchAll(/https?:\/\/[^\s"'`]+/g)) leakedCandidates.add(m[0]);
  }

  const outputs = [result.manifestJson, result.reportJson, ...result.partials.map((partial) => partial.markdown)];
  const blob = outputs.join("\n");

  for (const v of leakedCandidates) {
    if (allowedDefaults.has(v)) continue;
    assert.equal(blob.includes(v), false, `leaked value from config-*.toml: ${v}`);
  }

  // Pattern scan for addresses/URLs in defaults (zero-address ok if ever present)
  for (const k of manifest.keys) {
    if (k.default == null) continue;
    if (/^https?:\/\//i.test(k.default)) {
      assert.fail(`default for ${k.key} looks like a URL: ${k.default}`);
    }
    if (/^0x[a-fA-F0-9]{40}$/.test(k.default) && !/^0x0+$/i.test(k.default)) {
      assert.fail(`default for ${k.key} looks like a real address: ${k.default}`);
    }
  }
});

test("idempotent generate (twice, identical ephemeral output)", () => {
  const run = () => spawnSync("node", ["generate.js"], { cwd: TOOL_ROOT, encoding: "utf8", env: process.env });
  const a = run();
  assert.equal(a.status, 0, a.stderr || a.stdout);
  const snap = (dir) => {
    const out = {};
    function walk(d, prefix) {
      for (const e of fs.readdirSync(d, { withFileTypes: true })) {
        const rel = prefix ? `${prefix}/${e.name}` : e.name;
        const full = path.join(d, e.name);
        if (e.isDirectory()) walk(full, rel);
        else out[rel] = fs.readFileSync(full, "utf8");
      }
    }
    walk(dir, "");
    return out;
  };
  const before = snap(OUTPUT_DIR);
  const b = run();
  assert.equal(b.status, 0, b.stderr || b.stdout);
  const after = snap(OUTPUT_DIR);
  assert.deepEqual(after, before);
});
