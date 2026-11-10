import { isAddress } from "viem";

import { getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { formatEnvVarForLog, formatEnvVarValueForMessage } from "../../../common/helpers/envVarLogging.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `23_deploy_PaymentSplitterWrapper.ts` (same parsing and validation). */
export function paymentSplitterWrapperParameters(): ModuleParameters {
  const payeesRaw = getRequiredEnvVar("PAYMENT_SPLITTER_PAYEES");
  const sharesRaw = getRequiredEnvVar("PAYMENT_SPLITTER_SHARES");

  // --- Parse comma-separated strings into arrays ---
  const payeesArray = payeesRaw
    .split(",")
    .map((payee) => payee.trim())
    .filter((payee) => payee.length > 0);
  const sharesArray = sharesRaw
    .split(",")
    .map((share) => share.trim())
    .filter((share) => share.length > 0);

  // --- Validate arrays ---
  if (payeesArray.length === 0) {
    throw new Error("Payees array cannot be empty");
  }
  if (sharesArray.length === 0) {
    throw new Error("Shares array cannot be empty");
  }
  if (payeesArray.length !== sharesArray.length) {
    throw new Error(
      `Payees and shares arrays must have the same length. Got ${payeesArray.length} payees and ${sharesArray.length} shares`,
    );
  }

  // Validate addresses
  for (const [i, payee] of payeesArray.entries()) {
    if (!isAddress(payee)) {
      throw new Error(
        `Invalid address at index ${i}: ${formatEnvVarValueForMessage("PAYMENT_SPLITTER_PAYEES", payee)}`,
      );
    }
  }

  // Validate and parse shares
  const sharesBigInt: bigint[] = [];
  for (const [i, shareStr] of sharesArray.entries()) {
    const shareNum = BigInt(shareStr);
    if (shareNum <= 0n) {
      throw new Error(`Share at index ${i} must be positive: ${shareStr}`);
    }
    sharesBigInt.push(shareNum);
  }

  // --- Log params ---
  console.log("Params:");
  console.log(`  payees: ${formatEnvVarForLog("PAYMENT_SPLITTER_PAYEES", payeesRaw)}`);
  console.log(`  shares: ${formatEnvVarForLog("PAYMENT_SPLITTER_SHARES", sharesRaw)}`);

  return { payees: payeesArray, shares: sharesBigInt };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async () => ({
  PaymentSplitterWrapper: paymentSplitterWrapperParameters(),
});
