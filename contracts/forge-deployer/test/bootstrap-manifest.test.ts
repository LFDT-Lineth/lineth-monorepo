import assert from "node:assert/strict";
import test from "node:test";

import { bootstrapManifestHash, parseBootstrapManifest } from "../src/bootstrap-manifest";

const RAW_TX = `0x${"ab".repeat(64)}`;
const ADDRESS = "0x1000000000000000000000000000000000000001";

test("parses a manifest with all three item kinds", () => {
  const manifest = parseBootstrapManifest(
    JSON.stringify({
      version: 1,
      items: [
        { id: "fund-relayer", kind: "sign", chain: "l2", to: ADDRESS, valueWei: "1000" },
        { id: "deploy-factory", kind: "presigned", chain: "l2", rawTx: RAW_TX, expectAddress: ADDRESS },
        { id: "extra", kind: "script", chain: "l1", script: "bootstrap/extra.js" },
      ],
    }),
  );

  assert.equal(manifest.version, 1);
  assert.equal(manifest.items.length, 3);
  assert.equal(manifest.items[0]?.kind, "sign");
  assert.equal(manifest.items[1]?.kind, "presigned");
  assert.equal(manifest.items[2]?.kind, "script");
});

test("normalizes addresses to checksummed form and defaults valueWei to zero", () => {
  const manifest = parseBootstrapManifest(
    JSON.stringify({
      version: 1,
      items: [{ id: "fund", kind: "sign", chain: "l2", to: ADDRESS.toLowerCase() }],
    }),
  );
  const item = manifest.items[0];
  assert.equal(item?.kind, "sign");
  if (item?.kind === "sign") {
    assert.equal(item.to, ADDRESS);
    assert.equal(item.valueWei, "0");
  }
});

test("rejects a sign item with neither to nor data, and allows to+data (CREATE2 proxy call)", () => {
  assert.throws(
    () => parseBootstrapManifest(JSON.stringify({ version: 1, items: [{ id: "x", kind: "sign", chain: "l2" }] })),
    /must set "to" for a transfer\/call or "data" for a bare create/,
  );
  // to + data together is valid: a CREATE2 deploy through the deterministic
  // proxy is a normal signed call to the factory carrying the encoded deploy data.
  const proxyCall = parseBootstrapManifest(
    JSON.stringify({
      version: 1,
      items: [{ id: "x", kind: "sign", chain: "l2", to: ADDRESS, data: "0x1234", expectAddress: ADDRESS }],
    }),
  );
  assert.equal(proxyCall.items[0]?.kind, "sign");
  // A bare create (data, no to) is allowed; expectAddress is optional and, when
  // set, is asserted against receipt.contractAddress at execution time.
  const create = parseBootstrapManifest(
    JSON.stringify({ version: 1, items: [{ id: "x", kind: "sign", chain: "l2", data: "0x6000" }] }),
  );
  assert.equal(create.items[0]?.kind, "sign");
});

test("rejects duplicate ids, bad kinds, wrong version, and non-array items", () => {
  const base = { id: "x", kind: "presigned", chain: "l2", rawTx: RAW_TX };
  assert.throws(
    () => parseBootstrapManifest(JSON.stringify({ version: 1, items: [base, base] })),
    /duplicate item id x/,
  );
  assert.throws(
    () => parseBootstrapManifest(JSON.stringify({ version: 1, items: [{ ...base, kind: "nope" }] })),
    /kind must be "sign", "presigned", or "script"/,
  );
  assert.throws(() => parseBootstrapManifest(JSON.stringify({ version: 2, items: [] })), /version must be 1/);
  assert.throws(() => parseBootstrapManifest(JSON.stringify({ version: 1, items: {} })), /items must be an array/);
});

test("rejects malformed fields per kind", () => {
  assert.throws(
    () =>
      parseBootstrapManifest(
        JSON.stringify({ version: 1, items: [{ id: "BAD ID", kind: "script", chain: "l2", script: "s.js" }] }),
      ),
    /item id must be lowercase kebab-case/,
  );
  assert.throws(
    () =>
      parseBootstrapManifest(
        JSON.stringify({ version: 1, items: [{ id: "x", kind: "sign", chain: "l3", to: ADDRESS }] }),
      ),
    /chain must be "l1" or "l2"/,
  );
  assert.throws(
    () =>
      parseBootstrapManifest(
        JSON.stringify({ version: 1, items: [{ id: "x", kind: "presigned", chain: "l2", rawTx: "nothex" }] }),
      ),
    /rawTx must be a 0x-prefixed raw transaction/,
  );
  assert.throws(
    () =>
      parseBootstrapManifest(
        JSON.stringify({ version: 1, items: [{ id: "x", kind: "script", chain: "l2", script: "../escape.js" }] }),
      ),
    /must not contain "\.\."/,
  );
  assert.throws(
    () =>
      parseBootstrapManifest(
        JSON.stringify({ version: 1, items: [{ id: "x", kind: "sign", chain: "l2", to: "0x1234" }] }),
      ),
    /to must be an Ethereum address/,
  );
});

test("produces a stable, content-sensitive hash", () => {
  const one = parseBootstrapManifest(
    JSON.stringify({ version: 1, items: [{ id: "a", kind: "sign", chain: "l2", to: ADDRESS, valueWei: "1" }] }),
  );
  const same = parseBootstrapManifest(
    JSON.stringify({
      version: 1,
      items: [{ id: "a", kind: "sign", chain: "l2", to: ADDRESS.toLowerCase(), valueWei: "1" }],
    }),
  );
  const different = parseBootstrapManifest(
    JSON.stringify({ version: 1, items: [{ id: "a", kind: "sign", chain: "l2", to: ADDRESS, valueWei: "2" }] }),
  );

  assert.match(bootstrapManifestHash(one), /^0x[0-9a-f]{64}$/);
  // Address normalization makes equivalent manifests hash identically.
  assert.equal(bootstrapManifestHash(one), bootstrapManifestHash(same));
  assert.notEqual(bootstrapManifestHash(one), bootstrapManifestHash(different));
});
