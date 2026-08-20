import assert from "node:assert/strict";
import test from "node:test";

import { loadConfig, resolveRoleConfig, sanitizeText } from "../src/config";
import { assertDistinctChainIds } from "../src/runner";

const STATE_ROOT = `0x${"ab".repeat(32)}`;
const L1_ADDRESS = "0x1000000000000000000000000000000000000001";
const L2_ADDRESS = "0x2000000000000000000000000000000000000002";

function requiredEnvironment(): NodeJS.ProcessEnv {
  return {
    L1_RPC_URL: "https://user:password@l1.example/rpc",
    L2_RPC_URL: "http://l2.example:8545",
    L1_DEPLOYER_PRIVATE_KEY: "__test_l1_private_key__",
    L2_DEPLOYER_PRIVATE_KEY: "__test_l2_private_key__",
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

test("uses documented defaults and derives dev roles from deployers", () => {
  const config = loadConfig(requiredEnvironment());
  const roles = resolveRoleConfig(config, L1_ADDRESS, L2_ADDRESS);

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
