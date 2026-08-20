import { feeBudgetPricePerGas, FeeOverrides } from "../../common/helpers/feeOverrides";

export function assertDeployerCanPay(
  chain: "L1" | "L2",
  deployerAddress: string,
  balance: bigint,
  fees: FeeOverrides,
  gasBudget = 1n,
): void {
  const pricePerGas = feeBudgetPricePerGas(fees);
  const requiredBalance = pricePerGas * gasBudget;
  if (requiredBalance === 0n || balance >= requiredBalance) return;

  const l2Hint = chain === "L2" ? " or set L2_DEPLOY_GAS_PRICE_WEI=0 for a gas-free Forge network" : "";
  throw new Error(
    `Fund ${chain} deployer ${deployerAddress}; deployment requires at least ${requiredBalance} wei${l2Hint}`,
  );
}

export function formatInsufficientFundsError(chain: "L1" | "L2", deployerAddress: string, error: unknown): Error {
  const detail = error instanceof Error ? error.message : String(error);
  return new Error(`Fund ${chain} deployer ${deployerAddress}; deployment failed with: ${detail}`, {
    cause: error,
  });
}
