import assert from "node:assert/strict";
import test from "node:test";

import { loadConfig, resolveRoleConfig, sanitizeText } from "../src/config";
import { assertDistinctChainIds } from "../src/runner";

const STATE_ROOT = `0x${"ab".repeat(32)}`;
const IMAGE_DIGEST = `sha256:${"cd".repeat(32)}`;
const L1_ADDRESS = "0x1000000000000000000000000000000000000001";
const L2_ADDRESS = "0x2000000000000000000000000000000000000002";

function requiredEnvironment(): NodeJS.ProcessEnv {
  return {
    L1_RPC_URL: "https://user:password@l1.example/rpc",
    L2_RPC_URL: "http://l2.example:8545",
    L1_DEPLOYER_PRIVATE_KEY: "__test_l1_private_key__",
    L2_DEPLOYER_PRIVATE_KEY: "__test_l2_private_key__",
    L1_STARTING_NONCE: "0",
    L2_STARTING_NONCE: "0",
    DEPLOYER_IMAGE_DIGEST: IMAGE_DIGEST,
    INITIAL_L2_STATE_ROOT_HASH: STATE_ROOT,
  };
}

test("requires a 32-byte initial L2 state root", () => {
  const env = requiredEnvironment();
  env.INITIAL_L2_STATE_ROOT_HASH = "0x1234";

  assert.throws(() => loadConfig(env), /INITIAL_L2_STATE_ROOT_HASH must be a 32-byte hex value/);
});

test("validates uint256 bounds, positive rate limits, and nonzero role addresses", () => {
  const zeroPeriod = requiredEnvironment();
  zeroPeriod.CONTRACT_RATE_LIMIT_PERIOD = "0";
  assert.throws(() => loadConfig(zeroPeriod), /CONTRACT_RATE_LIMIT_PERIOD must be greater than zero/);

  const overflowAmount = requiredEnvironment();
  overflowAmount.CONTRACT_RATE_LIMIT_AMOUNT = (1n << 256n).toString();
  assert.throws(() => loadConfig(overflowAmount), /CONTRACT_RATE_LIMIT_AMOUNT must fit uint256/);

  const zeroRole = requiredEnvironment();
  zeroRole.L1_SECURITY_COUNCIL = "0x0000000000000000000000000000000000000000";
  assert.throws(() => loadConfig(zeroRole), /L1_SECURITY_COUNCIL must not be the zero address/);
});

test("rejects non-HTTP RPC endpoints", () => {
  const env = requiredEnvironment();
  env.L1_RPC_URL = "file:///tmp/not-an-rpc";

  assert.throws(() => loadConfig(env), /L1_RPC_URL must be an HTTP\(S\) URL/);
});

test("rejects identical L1 and L2 chain IDs", () => {
  assert.throws(() => assertDistinctChainIds("1337", "1337"), /L1 and L2 chain IDs must differ/);
  assert.doesNotThrow(() => assertDistinctChainIds("1", "1337"));
});

test("captures optional bootstrap manifest and scripts dir inputs", () => {
  const unset = loadConfig(requiredEnvironment());
  assert.equal(unset.bootstrapManifestFile, undefined);
  assert.equal(unset.bootstrapScriptsDir, undefined);

  const env = requiredEnvironment();
  env.BOOTSTRAP_MANIFEST_FILE = "/etc/bootstrap/manifest.json";
  env.BOOTSTRAP_SCRIPTS_DIR = "/etc/bootstrap/scripts";
  const config = loadConfig(env);
  assert.equal(config.bootstrapManifestFile, "/etc/bootstrap/manifest.json");
  assert.equal(config.bootstrapScriptsDir, "/etc/bootstrap/scripts");
});

test("requires pinned safe-integer starting nonces", () => {
  const missing = requiredEnvironment();
  delete missing.L1_STARTING_NONCE;
  assert.throws(() => loadConfig(missing), /L1_STARTING_NONCE is required/);

  const invalid = requiredEnvironment();
  invalid.L2_STARTING_NONCE = "-1";
  assert.throws(() => loadConfig(invalid), /L2_STARTING_NONCE must be a non-negative integer/);

  const overflow = requiredEnvironment();
  overflow.L1_STARTING_NONCE = (BigInt(Number.MAX_SAFE_INTEGER) + 1n).toString();
  assert.throws(() => loadConfig(overflow), /L1_STARTING_NONCE must fit a safe integer/);
});

test("requires an immutable deployment image digest", () => {
  const missing = requiredEnvironment();
  delete missing.DEPLOYER_IMAGE_DIGEST;
  assert.throws(() => loadConfig(missing), /DEPLOYER_IMAGE_DIGEST is required/);

  const malformed = requiredEnvironment();
  malformed.DEPLOYER_IMAGE_DIGEST = "sha256:1234";
  assert.throws(() => loadConfig(malformed), /DEPLOYER_IMAGE_DIGEST must be a sha256 digest/);

  const uppercase = requiredEnvironment();
  uppercase.DEPLOYER_IMAGE_DIGEST = `sha256:${"AB".repeat(32)}`;
  assert.throws(() => loadConfig(uppercase), /DEPLOYER_IMAGE_DIGEST must be a sha256 digest/);

  assert.equal(loadConfig(requiredEnvironment()).artifactDigest, IMAGE_DIGEST);
});

test("uses documented defaults and derives dev roles from deployers", () => {
  const config = loadConfig(requiredEnvironment());
  const roles = resolveRoleConfig(config, L1_ADDRESS, L2_ADDRESS);

  assert.equal(config.l1StartingNonce, 0);
  assert.equal(config.l2StartingNonce, 0);
  assert.equal(config.rateLimitPeriod, "86400");
  assert.equal(config.rateLimitAmount, "1000000000000000000000");
  assert.equal(roles.l1SecurityCouncil, L1_ADDRESS);
  assert.deepEqual(roles.l1RollupOperators, [L1_ADDRESS]);
  assert.equal(roles.l2SecurityCouncil, L2_ADDRESS);
  assert.equal(roles.l1L2MessageSetter, L2_ADDRESS);
});

test("redacts private keys and credential-bearing RPC URLs", () => {
  const config = loadConfig(requiredEnvironment());
  const text = `failed key=${config.l1PrivateKey} endpoint=${config.l1RpcUrl}`;

  assert.equal(sanitizeText(text, config.sensitiveValues), "failed key=[REDACTED] endpoint=[REDACTED]");
});

test("removes line separators from sanitized log text", () => {
  assert.equal(sanitizeText("failed\r\nforged\nentry\u2028tail\u2029end", []), "failed forged entry tail end");
});
