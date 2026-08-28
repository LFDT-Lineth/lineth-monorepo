import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";

import { BootstrapItem } from "../src/bootstrap-manifest";
import { orderBootstrapItems, resolveBootstrapScriptPath } from "../src/steps/bootstrap-helpers";

test("resolveBootstrapScriptPath accepts a script inside scriptsDir", () => {
  const scriptsDir = "/bootstrap/scripts";
  const resolved = resolveBootstrapScriptPath(scriptsDir, "item-1", "deploy.js");
  assert.equal(resolved, path.resolve(scriptsDir, "deploy.js"));
});

test("resolveBootstrapScriptPath accepts a script in a nested subdirectory", () => {
  const scriptsDir = "/bootstrap/scripts";
  const resolved = resolveBootstrapScriptPath(scriptsDir, "item-1", "nested/deploy.js");
  assert.equal(resolved, path.resolve(scriptsDir, "nested/deploy.js"));
});

test("resolveBootstrapScriptPath rejects a script that escapes scriptsDir via ../", () => {
  assert.throws(
    () => resolveBootstrapScriptPath("/bootstrap/scripts", "item-1", "../outside.js"),
    /resolves outside BOOTSTRAP_SCRIPTS_DIR/,
  );
});

test("resolveBootstrapScriptPath rejects an absolute path outside scriptsDir", () => {
  assert.throws(
    () => resolveBootstrapScriptPath("/bootstrap/scripts", "item-1", "/etc/passwd"),
    /resolves outside BOOTSTRAP_SCRIPTS_DIR/,
  );
});

test("resolveBootstrapScriptPath rejects a sibling directory with a matching name prefix", () => {
  // "/bootstrap/scripts-evil/x.js" starts with the string "/bootstrap/scripts"
  // but is not inside it; the check must compare against root + path.sep.
  assert.throws(
    () => resolveBootstrapScriptPath("/bootstrap/scripts", "item-1", "../scripts-evil/x.js"),
    /resolves outside BOOTSTRAP_SCRIPTS_DIR/,
  );
});

function signItem(id: string): BootstrapItem {
  return { kind: "sign", id, chain: "l2", valueWei: "0" };
}
function presignedItem(id: string): BootstrapItem {
  return { kind: "presigned", id, chain: "l2", rawTx: "0x" };
}
function scriptItem(id: string): BootstrapItem {
  return { kind: "script", id, chain: "l2", script: "x.js" };
}

test("orderBootstrapItems runs sign items first, then presigned, then scripts", () => {
  const items: BootstrapItem[] = [scriptItem("s1"), presignedItem("p1"), signItem("sign1"), signItem("sign2")];
  const ordered = orderBootstrapItems(items);
  assert.deepEqual(
    ordered.map((item) => item.id),
    ["sign1", "sign2", "p1", "s1"],
  );
});

test("orderBootstrapItems preserves relative order within each kind", () => {
  const items: BootstrapItem[] = [signItem("sign2"), signItem("sign1"), scriptItem("s2"), scriptItem("s1")];
  const ordered = orderBootstrapItems(items);
  assert.deepEqual(
    ordered.map((item) => item.id),
    ["sign2", "sign1", "s2", "s1"],
  );
});

test("orderBootstrapItems is a no-op on an already-ordered or empty list", () => {
  assert.deepEqual(orderBootstrapItems([]), []);
});
