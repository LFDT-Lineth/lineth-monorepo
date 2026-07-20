const { test } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const { spawnSync } = require("node:child_process");

const { escapeCell, isNeutralPartial, checkCompleteness, build, partialComponentName } = require("./lib");
const { cleanDescription, isDevNoiseComment, extract } = require("./parse-go");
const {
  TOOL_ROOT,
  MONOREPO_ROOT,
  OUTPUT_DIR,
  GENERATED_DIR,
  MANIFEST_PATH,
  REPORT_PATH,
  WRAPPER_TEMPLATE_PATH,
} = require("./paths");

test("escapeCell escapes MDX-sensitive characters", () => {
  assert.equal(escapeCell("a|b"), "a\\|b");
  assert.equal(escapeCell("a{b}"), "a\\{b\\}");
  assert.equal(escapeCell("<x>"), "&lt;x&gt;");
});

test("dev-noise comments are stripped from descriptions", () => {
  assert.equal(isDevNoiseComment("TODO @gbotrel fix this"), true);
  assert.equal(isDevNoiseComment("not serialized"), true);
  assert.equal(isDevNoiseComment("for testing purposes only"), true);
  assert.equal(isDevNoiseComment("duplicate from Config.Layer2"), true);
  assert.equal(isDevNoiseComment("The delays at which we retry"), false);
  assert.equal(cleanDescription(["// TODO @gbotrel noise", "// Real description here"]), "Real description here");
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

const hasCommittedOutput = fs.existsSync(MANIFEST_PATH);

test("extract discovers documentable keys and excludes env-only/-", { skip: !hasCommittedOutput }, () => {
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

test("rendered rows match documentable key count", { skip: !hasCommittedOutput }, async () => {
  const result = await build({ monorepoRoot: MONOREPO_ROOT, toolRoot: TOOL_ROOT });
  assert.equal(result.rowCount, result.manifest.counts.total);
  assert.equal(result.rowCount, result.manifest.keys.length);
});

test("spot-check known defaults and allowed values", { skip: !hasCommittedOutput }, () => {
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

test("partials are neutral and traces-limits is a note", { skip: !hasCommittedOutput }, () => {
  assert.ok(fs.existsSync(GENERATED_DIR));
  const files = fs.readdirSync(GENERATED_DIR).filter((f) => f.endsWith(".mdx"));
  assert.ok(files.includes("traces-limits.mdx"));
  for (const f of files) {
    const md = fs.readFileSync(path.join(GENERATED_DIR, f), "utf8");
    assert.equal(isNeutralPartial(md), true, `${f} should be neutral`);
  }
  const note = fs.readFileSync(path.join(GENERATED_DIR, "traces-limits.mdx"), "utf8");
  assert.match(note, /prefix/i);
  assert.doesNotMatch(note, /\| Config key \|/);
});

test("wrapper completeness against committed partials", { skip: !hasCommittedOutput }, () => {
  assert.ok(fs.existsSync(WRAPPER_TEMPLATE_PATH));
  const wrapper = fs.readFileSync(WRAPPER_TEMPLATE_PATH, "utf8");
  const partials = fs
    .readdirSync(GENERATED_DIR)
    .filter((f) => f.endsWith(".mdx"))
    .sort();
  const failures = checkCompleteness(wrapper, partials);
  assert.deepEqual(failures, []);
});

test("output is public-safe: no config-*.toml secrets leaked", { skip: !hasCommittedOutput }, () => {
  const configDir = path.join(MONOREPO_ROOT, "prover", "config");
  const tomlFiles = fs.readdirSync(configDir).filter((f) => f.startsWith("config-") && f.endsWith(".toml"));
  assert.ok(tomlFiles.length > 0);

  // Intentional defaults from config_default.go (paths, cmds) are allowed.
  const { manifest } = extract(MONOREPO_ROOT);
  const allowedDefaults = new Set(manifest.keys.map((k) => k.default).filter((d) => d != null && d !== ""));

  const leakedCandidates = new Set();
  for (const f of tomlFiles) {
    const text = fs.readFileSync(path.join(configDir, f), "utf8");
    for (const m of text.matchAll(/0x[a-fA-F0-9]{40}/g)) leakedCandidates.add(m[0]);
    for (const m of text.matchAll(/https?:\/\/[^\s"'`]+/g)) leakedCandidates.add(m[0]);
  }

  const outputs = [];
  outputs.push(fs.readFileSync(MANIFEST_PATH, "utf8"));
  outputs.push(fs.readFileSync(REPORT_PATH, "utf8"));
  for (const f of fs.readdirSync(GENERATED_DIR)) {
    if (f.endsWith(".mdx")) outputs.push(fs.readFileSync(path.join(GENERATED_DIR, f), "utf8"));
  }
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

test("idempotent generate (twice, no diff)", { skip: !hasCommittedOutput }, () => {
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
