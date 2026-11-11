import { getRequiredEnvVar } from "../../../common/helpers/environment.js";
import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";
import { toUint } from "../lib/parameters.js";

import type { DeployCompanion } from "../../../hardhat-plugins/deploy/types.js";

/** Same inputs as the Hardhat 2 `18_deploy_RollupRevenueVaultWithReinitialization.ts` script. */
export const parameters: NonNullable<DeployCompanion["parameters"]> = async ({ networkName }) => ({
  RollupRevenueVaultReinitialize: {
    proxy: requireAddressFromRegistryOrEnv(networkName, "RollupRevenueVault", "ROLLUP_REVENUE_VAULT_ADDRESS"),
    lastInvoiceDate: toUint(
      "ROLLUP_REVENUE_VAULT_LAST_INVOICE_DATE",
      getRequiredEnvVar("ROLLUP_REVENUE_VAULT_LAST_INVOICE_DATE"),
    ),
    securityCouncil: requireAddressFromRegistryOrEnv(networkName, "L2_SECURITY_COUNCIL", "L2_SECURITY_COUNCIL"),
    invoiceSubmitter: requireAddressFromRegistryOrEnv(
      networkName,
      "ROLLUP_REVENUE_VAULT_INVOICE_SUBMITTER",
      "ROLLUP_REVENUE_VAULT_INVOICE_SUBMITTER",
    ),
    burner: requireAddressFromRegistryOrEnv(networkName, "ROLLUP_REVENUE_VAULT_BURNER", "ROLLUP_REVENUE_VAULT_BURNER"),
    invoicePaymentReceiver: requireAddressFromRegistryOrEnv(
      networkName,
      "ROLLUP_REVENUE_VAULT_INVOICE_PAYMENT_RECEIVER",
      "ROLLUP_REVENUE_VAULT_INVOICE_PAYMENT_RECEIVER",
    ),
    tokenBridge: requireAddressFromRegistryOrEnv(networkName, "TokenBridge_L2", "TOKEN_BRIDGE_ADDRESS"),
    messageService: requireAddressFromRegistryOrEnv(networkName, "L2MessageService", "L2_MESSAGE_SERVICE_ADDRESS"),
    l1LineaTokenBurner: requireAddressFromRegistryOrEnv(
      networkName,
      "ROLLUP_REVENUE_VAULT_L1_LINEA_TOKEN_BURNER",
      "ROLLUP_REVENUE_VAULT_L1_LINEA_TOKEN_BURNER",
    ),
    lineaToken: requireAddressFromRegistryOrEnv(
      networkName,
      "ROLLUP_REVENUE_VAULT_LINEA_TOKEN",
      "ROLLUP_REVENUE_VAULT_LINEA_TOKEN",
    ),
    dexSwapAdapter: requireAddressFromRegistryOrEnv(
      networkName,
      "ROLLUP_REVENUE_VAULT_DEX_SWAP_ADAPTER",
      "ROLLUP_REVENUE_VAULT_DEX_SWAP_ADAPTER",
    ),
  },
});
