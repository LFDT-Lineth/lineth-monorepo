import { ethers } from "ethers";
import assert from "node:assert/strict";
import test from "node:test";

import {
  ARACHNID_FACTORY,
  ARACHNID_FACTORY_RUNTIME_CODE,
  ARACHNID_FACTORY_RUNTIME_CODE_HASH,
  ARACHNID_RAW_TX,
  ARACHNID_SIGNER,
  getDeterministicProxyCodeStatus,
  isDeterministicProxyDeployed,
} from "../../common/helpers/deterministicDeploymentProxy";

const UNRELATED_CODE = "0x6001600101";

function mockProvider(code: string): ethers.Provider {
  return {
    getCode: async () => code,
  } as unknown as ethers.Provider;
}

test("ARACHNID_RAW_TX recovers to ARACHNID_SIGNER", () => {
  const recovered = ethers.Transaction.from(ARACHNID_RAW_TX).from;
  assert.ok(recovered);
  assert.equal(ethers.getAddress(recovered), ARACHNID_SIGNER);
});

test("ARACHNID_FACTORY_RUNTIME_CODE_HASH matches the runtime code embedded in ARACHNID_RAW_TX's init code", () => {
  // The deployment transaction's init code is `PUSH1 0x45 DUP1 PUSH1 0x0e
  // PUSH1 0x00 CODECOPY DUP1 PUSH1 0x00 RETURN POP INVALID <runtime code>`:
  // 10 bytes of deploy preamble (offset 0x0e) followed by 0x45 (69) bytes of
  // runtime code that CODECOPY/RETURN install as the factory's deployed code.
  const tx = ethers.Transaction.from(ARACHNID_RAW_TX);
  const initCode = ethers.getBytes(tx.data);
  const runtimeCode = ethers.hexlify(initCode.subarray(14, 14 + 0x45));
  assert.equal(runtimeCode, ARACHNID_FACTORY_RUNTIME_CODE);
  assert.equal(ethers.keccak256(runtimeCode), ARACHNID_FACTORY_RUNTIME_CODE_HASH);
});

test("getDeterministicProxyCodeStatus reports absent when no code exists", async () => {
  const status = await getDeterministicProxyCodeStatus(mockProvider("0x"));
  assert.equal(status, "absent");
  assert.equal(await isDeterministicProxyDeployed(mockProvider("0x")), false);
});

test("getDeterministicProxyCodeStatus reports match for the known factory runtime code", async () => {
  const status = await getDeterministicProxyCodeStatus(mockProvider(ARACHNID_FACTORY_RUNTIME_CODE));
  assert.equal(status, "match");
  assert.equal(await isDeterministicProxyDeployed(mockProvider(ARACHNID_FACTORY_RUNTIME_CODE)), true);
});

test("getDeterministicProxyCodeStatus reports mismatch for unrelated non-empty code", async () => {
  const status = await getDeterministicProxyCodeStatus(mockProvider(UNRELATED_CODE));
  assert.equal(status, "mismatch");
  // A mismatch is still non-empty code, so isDeterministicProxyDeployed (a
  // coarse "is something there" check) reports true; callers that need to
  // distinguish "safe to adopt" from "unexpected occupant" must use
  // getDeterministicProxyCodeStatus directly, as ensureDeterministicDeploymentProxy
  // and the forge-deployer recovery path both do.
  assert.equal(await isDeterministicProxyDeployed(mockProvider(UNRELATED_CODE)), true);
});

test("ARACHNID_FACTORY is the canonical well-known address", () => {
  assert.equal(ARACHNID_FACTORY, ethers.getAddress("0x4e59b44847b379578588920cA78FbF26c0B4956C"));
});
