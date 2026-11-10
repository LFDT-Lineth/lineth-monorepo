import { getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";
import { toUint } from "../lib/parameters.js";

import type { DeployCompanion, ParameterContext } from "../../../hardhat-plugins/deploy/types.js";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";

/** `18_deploy_RollupRevenueVault.ts`. */
export function rollupRevenueVaultParameters(context: ParameterContext): ModuleParameters {
  const { networkName } = context;
  const lastInvoiceDate = getRequiredEnvVar("ROLLUP_REVENUE_VAULT_LAST_INVOICE_DATE");
  const securityCouncil = requireAddressFromRegistryOrEnv(networkName, "L2_SECURITY_COUNCIL", "L2_SECURITY_COUNCIL");
  const invoiceSubmitter = requireAddressFromRegistryOrEnv(
    networkName,
    "ROLLUP_REVENUE_VAULT_INVOICE_SUBMITTER",
    "ROLLUP_REVENUE_VAULT_INVOICE_SUBMITTER",
  );
  const burner = requireAddressFromRegistryOrEnv(
    networkName,
    "ROLLUP_REVENUE_VAULT_BURNER",
    "ROLLUP_REVENUE_VAULT_BURNER",
  );
  const invoicePaymentReceiver = requireAddressFromRegistryOrEnv(
    networkName,
    "ROLLUP_REVENUE_VAULT_INVOICE_PAYMENT_RECEIVER",
    "ROLLUP_REVENUE_VAULT_INVOICE_PAYMENT_RECEIVER",
  );
  const tokenBridge = requireAddressFromRegistryOrEnv(networkName, "TokenBridge_L2", "TOKEN_BRIDGE_ADDRESS");
  const messageService = requireAddressFromRegistryOrEnv(networkName, "L2MessageService", "L2_MESSAGE_SERVICE_ADDRESS");
  const l1LineaTokenBurner = requireAddressFromRegistryOrEnv(
    networkName,
    "ROLLUP_REVENUE_VAULT_L1_LINEA_TOKEN_BURNER",
    "ROLLUP_REVENUE_VAULT_L1_LINEA_TOKEN_BURNER",
  );
  const lineaToken = requireAddressFromRegistryOrEnv(
    networkName,
    "ROLLUP_REVENUE_VAULT_LINEA_TOKEN",
    "ROLLUP_REVENUE_VAULT_LINEA_TOKEN",
  );
  const dexSwapAdapter = requireAddressFromRegistryOrEnv(
    networkName,
    "ROLLUP_REVENUE_VAULT_DEX_SWAP_ADAPTER",
    "ROLLUP_REVENUE_VAULT_DEX_SWAP_ADAPTER",
  );

  return {
    lastInvoiceDate: toUint("ROLLUP_REVENUE_VAULT_LAST_INVOICE_DATE", lastInvoiceDate),
    securityCouncil,
    invoiceSubmitter,
    burner,
    invoicePaymentReceiver,
    tokenBridge,
    messageService,
    l1LineaTokenBurner,
    lineaToken,
    dexSwapAdapter,
  };
}

export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => ({
  RollupRevenueVault: rollupRevenueVaultParameters(context),
});
