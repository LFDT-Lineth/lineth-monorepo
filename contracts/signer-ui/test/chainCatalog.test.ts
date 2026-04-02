import assert from "node:assert/strict";
import test from "node:test";

import { SIGNER_UI_CHAIN_CATALOG, getSignerUiChainCatalogEntry } from "../shared/chainCatalog.ts";

test("chain catalog is non-empty and has unique numeric chain ids", () => {
  assert.ok(SIGNER_UI_CHAIN_CATALOG.length > 0);

  const ids = SIGNER_UI_CHAIN_CATALOG.map((entry) => entry.chainId);
  const unique = new Set(ids);
  assert.equal(unique.size, ids.length);

  for (const entry of SIGNER_UI_CHAIN_CATALOG) {
    assert.ok(Number.isInteger(entry.chainId));
    assert.ok(entry.chainName.length > 0);
    assert.ok(entry.rpcUrls.length > 0);
    assert.ok(entry.nativeCurrency.symbol.length > 0);
  }
});

test("lookup by chain id returns expected catalog entries", () => {
  const mainnet = getSignerUiChainCatalogEntry(1);
  assert.ok(mainnet);
  assert.equal(mainnet.chainName, "Ethereum Mainnet");

  const unknown = getSignerUiChainCatalogEntry(123456789);
  assert.equal(unknown, undefined);
});
