import assert from "node:assert/strict";
import test from "node:test";

import { ARACHNID_FUNDING_WEI } from "../../common/helpers/deterministicDeploymentProxy";
import { resolveL2DeployFeeOverrides } from "../../common/helpers/feeOverrides";
import { assertDeployerCanPay, formatInsufficientFundsError } from "../src/funds";

const DEPLOYER = "0x2000000000000000000000000000000000000002";

test("honors an explicit zero L2 deployment gas price", () => {
  const previous = process.env.L2_DEPLOY_GAS_PRICE_WEI;
  process.env.L2_DEPLOY_GAS_PRICE_WEI = "0";
  try {
    assert.deepEqual(resolveL2DeployFeeOverrides(), { gasPrice: 0n });
  } finally {
    if (previous === undefined) delete process.env.L2_DEPLOY_GAS_PRICE_WEI;
    else process.env.L2_DEPLOY_GAS_PRICE_WEI = previous;
  }
});

test("does not require a positive balance when the selected fee is zero", () => {
  assert.doesNotThrow(() => assertDeployerCanPay("L2", DEPLOYER, 0n, { gasPrice: 0n }));
});

test("reports actionable insufficient funds on a fee-paying chain", () => {
  assert.throws(
    () => assertDeployerCanPay("L2", DEPLOYER, 0n, { gasPrice: 1n }),
    /Fund L2 deployer 0x2000.*or set L2_DEPLOY_GAS_PRICE_WEI=0/,
  );
  assert.match(
    formatInsufficientFundsError("L1", DEPLOYER, new Error("insufficient funds for gas * price + value")).message,
    /Fund L1 deployer/,
  );
});

test("requires the full configured deployment gas budget", () => {
  assert.throws(() => assertDeployerCanPay("L1", DEPLOYER, 99n, { gasPrice: 10n }, 10n), /requires at least 100 wei/);
  assert.doesNotThrow(() => assertDeployerCanPay("L1", DEPLOYER, 100n, { gasPrice: 10n }, 10n));
});

test("a gas-free L2 still requires the deterministic proxy funding amount", () => {
  // Zero gas price must not be read as "no balance needed": the proxy step
  // funds the keyless signer with ARACHNID_FUNDING_WEI even when gas is free.
  assert.throws(
    () => assertDeployerCanPay("L2", DEPLOYER, 0n, { gasPrice: 0n }, 1n, ARACHNID_FUNDING_WEI),
    /requires at least 10000000000000000 wei/,
  );
  assert.throws(
    () => assertDeployerCanPay("L2", DEPLOYER, ARACHNID_FUNDING_WEI - 1n, { gasPrice: 0n }, 1n, ARACHNID_FUNDING_WEI),
    /requires at least 10000000000000000 wei/,
  );
  assert.doesNotThrow(() =>
    assertDeployerCanPay("L2", DEPLOYER, ARACHNID_FUNDING_WEI, { gasPrice: 0n }, 1n, ARACHNID_FUNDING_WEI),
  );
});

test("flat funding wei is added on top of the gas budget on a fee-paying chain", () => {
  // 10 wei/gas * 10 gas = 100 wei gas budget, plus 0.01 ETH flat funding.
  const required = 100n + ARACHNID_FUNDING_WEI;
  assert.throws(
    () => assertDeployerCanPay("L2", DEPLOYER, required - 1n, { gasPrice: 10n }, 10n, ARACHNID_FUNDING_WEI),
    new RegExp(`requires at least ${required} wei`),
  );
  assert.doesNotThrow(() =>
    assertDeployerCanPay("L2", DEPLOYER, required, { gasPrice: 10n }, 10n, ARACHNID_FUNDING_WEI),
  );
});
