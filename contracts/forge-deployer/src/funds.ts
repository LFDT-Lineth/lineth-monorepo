import { feeBudgetPricePerGas, FeeOverrides } from "../../common/helpers/feeOverrides";

/**
 * Whether an L2 preflight funding check needs the deployer's real on-chain
 * balance. Zero gas price only means "no balance needed" when there is also
 * no flat transfer to make (e.g. the deterministic proxy's funding send): a
 * gas-free L2 that still owes `flatWei` must have its real balance checked,
 * otherwise callers that substitute a fake zero balance for a zero-gas chain
 * would always fail this check — even when the deployer already holds
 * enough to cover `flatWei` — because zero can never satisfy a positive
 * requirement.
 */
export function requiresL2BalanceCheck(fees: FeeOverrides, flatWei: bigint): boolean {
  return feeBudgetPricePerGas(fees) !== 0n || flatWei !== 0n;
}

export function assertDeployerCanPay(
  chain: "L1" | "L2",
  deployerAddress: string,
  balance: bigint,
  fees: FeeOverrides,
  gasBudget = 1n,
  // Flat wei the deployment must send on top of gas, e.g. the deterministic
  // proxy's funding transfer to the keyless signer. On a gas-free chain the
  // gas term is 0, so without this an unfunded deployer would pass preflight
  // and only fail mid-step, after an in-flight intent is already persisted.
  flatWei = 0n,
): void {
  const pricePerGas = feeBudgetPricePerGas(fees);
  const requiredBalance = pricePerGas * gasBudget + flatWei;
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
