import { network } from "hardhat";
import { DeployFunction } from "hardhat-deploy/types";

import { ROLLUP_REVENUE_VAULT_INITIALIZE_SIGNATURE } from "../common/constants";
import {
  tryVerifyContract,
  LogContractDeployment,
  getRequiredEnvVar,
  requireAddressFromRegistryOrEnv,
} from "../common/helpers";
import { withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";
import { deployUpgradableFromFactory } from "../scripts/hardhat/utils";

const func: DeployFunction = withSignerUiSession("18_deploy_RollupRevenueVault.ts", async function () {
  const contractName = "RollupRevenueVault";

  const lastInvoiceDate = getRequiredEnvVar("ROLLUP_REVENUE_VAULT_LAST_INVOICE_DATE");
  const securityCouncil = requireAddressFromRegistryOrEnv(network.name, "L2_SECURITY_COUNCIL", "L2_SECURITY_COUNCIL");
  const invoiceSubmitter = requireAddressFromRegistryOrEnv(
    network.name,
    "ROLLUP_REVENUE_VAULT_INVOICE_SUBMITTER",
    "ROLLUP_REVENUE_VAULT_INVOICE_SUBMITTER",
  );
  const burner = requireAddressFromRegistryOrEnv(
    network.name,
    "ROLLUP_REVENUE_VAULT_BURNER",
    "ROLLUP_REVENUE_VAULT_BURNER",
  );
  const invoicePaymentReceiver = requireAddressFromRegistryOrEnv(
    network.name,
    "ROLLUP_REVENUE_VAULT_INVOICE_PAYMENT_RECEIVER",
    "ROLLUP_REVENUE_VAULT_INVOICE_PAYMENT_RECEIVER",
  );
  const tokenBridge = requireAddressFromRegistryOrEnv(network.name, "TokenBridge_L2", "TOKEN_BRIDGE_ADDRESS");
  const messageService = requireAddressFromRegistryOrEnv(
    network.name,
    "L2MessageService",
    "L2_MESSAGE_SERVICE_ADDRESS",
  );
  const l1LineaTokenBurner = requireAddressFromRegistryOrEnv(
    network.name,
    "ROLLUP_REVENUE_VAULT_L1_LINEA_TOKEN_BURNER",
    "ROLLUP_REVENUE_VAULT_L1_LINEA_TOKEN_BURNER",
  );
  const lineaToken = requireAddressFromRegistryOrEnv(
    network.name,
    "ROLLUP_REVENUE_VAULT_LINEA_TOKEN",
    "ROLLUP_REVENUE_VAULT_LINEA_TOKEN",
  );
  const dexSwapAdapter = requireAddressFromRegistryOrEnv(
    network.name,
    "ROLLUP_REVENUE_VAULT_DEX_SWAP_ADAPTER",
    "ROLLUP_REVENUE_VAULT_DEX_SWAP_ADAPTER",
  );

  const contract = await deployUpgradableFromFactory(
    contractName,
    [
      lastInvoiceDate,
      securityCouncil,
      invoiceSubmitter,
      burner,
      invoicePaymentReceiver,
      tokenBridge,
      messageService,
      l1LineaTokenBurner,
      lineaToken,
      dexSwapAdapter,
    ],
    {
      initializer: ROLLUP_REVENUE_VAULT_INITIALIZE_SIGNATURE,
      unsafeAllow: ["constructor", "incorrect-initializer-order"],
    },
  );

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryVerifyContract(contractAddress, "src/operational/RollupRevenueVault.sol:RollupRevenueVault");
});

export default func;
func.tags = ["RollupRevenueVault"];
