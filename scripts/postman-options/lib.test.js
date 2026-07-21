const { test } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

const { escapeCell, isNeutralPartial, checkCompleteness, build, partialComponentName } = require("./lib");
const { extract, expandInventoryEntry, verifyDefaultKinds } = require("./parse-postman");
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

test("partialComponentName", () => {
  assert.equal(partialComponentName("l1-network"), "L1Network");
  assert.equal(partialComponentName("database-cleaner"), "DatabaseCleaner");
});

test("checkCompleteness fails when a partial is missing from the wrapper", () => {
  const failures = checkCompleteness("import Foo from './_generated/postman/foo.mdx';\n<Foo />\n", [
    "foo.mdx",
    "bar.mdx",
  ]);
  assert.ok(failures.some((f) => f.includes("bar.mdx")));
});

test("checkCompleteness allows publish-only provenance.mdx", () => {
  const wrapper =
    "import Provenance from './_generated/postman/provenance.mdx';\n" +
    "import Foo from './_generated/postman/foo.mdx';\n" +
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

test("extract discovers env vars after L1/L2 expansion", { skip: !hasCommittedOutput }, () => {
  const { manifest, report } = extract(MONOREPO_ROOT);
  assert.ok(manifest.keys.length >= 70, `expected many env vars, got ${manifest.keys.length}`);
  const keySet = new Set(manifest.keys.map((k) => k.envVar));
  assert.equal(keySet.has("L1_LISTENER_INTERVAL"), true);
  assert.equal(keySet.has("L2_LISTENER_INTERVAL"), true);
  assert.equal(keySet.has("L1_L2_ENABLE_POSTMAN_SPONSORING"), true);
  assert.equal(keySet.has("L2_L1_ENABLE_POSTMAN_SPONSORING"), true);
  assert.equal(keySet.has("MAX_NONCE_DIFF"), true);
  assert.equal(report.inventoryGaps.missingFromInventory.length, 0);
  assert.equal(report.inventoryGaps.extraInInventory.length, 0);
  // passthrough DB leaves have blank descriptions
  assert.ok(report.missingDescriptions.some((m) => m.envVar === "POSTGRES_HOST"));
});

test("rendered rows match documentable env-var count", { skip: !hasCommittedOutput }, async () => {
  const result = await build({ monorepoRoot: MONOREPO_ROOT, toolRoot: TOOL_ROOT });
  assert.equal(result.rowCount, result.manifest.counts.total);
  assert.equal(result.rowCount, result.manifest.keys.length);
});

test("spot-check known defaults, types, and descriptions", { skip: !hasCommittedOutput }, () => {
  const { manifest } = extract(MONOREPO_ROOT);
  const byKey = new Map(manifest.keys.map((k) => [k.envVar, k]));

  assert.equal(byKey.get("LOG_LEVEL").default, "info");
  assert.match(byKey.get("LOG_LEVEL").type, /string/);
  assert.match(byKey.get("LOG_LEVEL").description, /log level/i);
  assert.doesNotMatch(byKey.get("LOG_LEVEL").type, /\bany\b/);
  assert.equal(byKey.get("POSTGRES_PORT").default, "5432");

  const rpc = byKey.get("L1_RPC_URL");
  assert.equal(rpc.default, null);
  assert.equal(rpc.required, true);
  assert.match(rpc.description, /JSON-RPC/i);
  assert.match(rpc.type, /url/i);

  const nonce = byKey.get("MAX_NONCE_DIFF");
  assert.equal(nonce.default, null);
  assert.equal(nonce.required, false);
  assert.match(nonce.type, /number/);
  assert.match(nonce.description, /nonce/i);

  const signerType = byKey.get("L1_SIGNER_TYPE");
  assert.equal(signerType.default, "private-key");
  assert.deepEqual(signerType.oneof, ["private-key", "web3signer", "aws-kms"]);

  const pk = byKey.get("L1_SIGNER_PRIVATE_KEY");
  assert.equal(pk.default, null);
  assert.ok(pk.secret);
  assert.ok(pk.description.length > 0);
  assert.match(pk.requiredLabel, /L1_SIGNER_TYPE/);

  const pk2 = byKey.get("L2_SIGNER_PRIVATE_KEY");
  assert.match(pk2.requiredLabel, /L2_SIGNER_TYPE/);

  assert.equal(byKey.get("L1_L2_AUTO_CLAIM_ENABLED").default, "false");
  assert.equal(byKey.get("L1_L2_AUTO_CLAIM_ENABLED").required, true);
});

test("verifyDefaultKinds flags literal mismatch against envLoader source", () => {
  const src = 'level: process.env.LOG_LEVEL ?? "info",\n';
  const ok = verifyDefaultKinds(src, [{ env: "LOG_LEVEL", defaultKind: "literal:info" }]);
  assert.deepEqual(ok, []);

  const bad = verifyDefaultKinds(src, [{ env: "LOG_LEVEL", defaultKind: "literal:debug" }]);
  assert.equal(bad.length, 1);
  assert.match(bad[0], /LOG_LEVEL/);
});

test("expandInventoryEntry prefixes SIGNER_TYPE in conditions", () => {
  const [l1, l2] = expandInventoryEntry({
    expand: "prefix",
    envSuffix: "SIGNER_PRIVATE_KEY",
    condition: 'used when SIGNER_TYPE is "private-key"',
    defaultKind: "placeholder:0x",
  });
  assert.equal(l1.env, "L1_SIGNER_PRIVATE_KEY");
  assert.equal(l1.condition, 'used when L1_SIGNER_TYPE is "private-key"');
  assert.equal(l2.condition, 'used when L2_SIGNER_TYPE is "private-key"');
});

test("partials are neutral tables", { skip: !hasCommittedOutput }, () => {
  assert.ok(fs.existsSync(GENERATED_DIR));
  const files = fs.readdirSync(GENERATED_DIR).filter((f) => f.endsWith(".mdx"));
  assert.ok(files.includes("general.mdx"));
  assert.ok(files.includes("signer.mdx"));
  for (const f of files) {
    const md = fs.readFileSync(path.join(GENERATED_DIR, f), "utf8");
    assert.equal(isNeutralPartial(md), true, `${f} should be neutral`);
    assert.match(md, /\|\s*Env var\s*\|/);
  }
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

test("output is public-safe: no addresses, endpoints, or private-key values", { skip: !hasCommittedOutput }, () => {
  const { manifest } = extract(MONOREPO_ROOT);

  const outputs = [];
  outputs.push(fs.readFileSync(MANIFEST_PATH, "utf8"));
  outputs.push(fs.readFileSync(REPORT_PATH, "utf8"));
  for (const f of fs.readdirSync(GENERATED_DIR)) {
    if (f.endsWith(".mdx")) outputs.push(fs.readFileSync(path.join(GENERATED_DIR, f), "utf8"));
  }
  const blob = outputs.join("\n");

  assert.doesNotMatch(blob, /https?:\/\//i, "output must not contain http(s) endpoints");
  // Non-zero 40-char addresses
  assert.doesNotMatch(blob, /0x[a-fA-F0-9]{40}(?![a-fA-F0-9])/);
  // 64-hex private-key-looking blobs (with or without 0x)
  assert.doesNotMatch(blob, /(?:0x)?[a-fA-F0-9]{64}(?![a-fA-F0-9])/);

  for (const k of manifest.keys) {
    if (k.default == null) continue;
    if (/^https?:\/\//i.test(k.default)) {
      assert.fail(`default for ${k.envVar} looks like a URL: ${k.default}`);
    }
    if (/^0x[a-fA-F0-9]{40}$/.test(k.default) && !/^0x0+$/i.test(k.default)) {
      assert.fail(`default for ${k.envVar} looks like a real address: ${k.default}`);
    }
    if (/^(?:0x)?[a-fA-F0-9]{64}$/.test(k.default)) {
      assert.fail(`default for ${k.envVar} looks like a private key`);
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
