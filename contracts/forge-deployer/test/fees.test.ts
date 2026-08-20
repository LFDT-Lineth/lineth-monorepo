import assert from "node:assert/strict";
import test from "node:test";

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
