import { DeployFunction } from "hardhat-deploy/types";
import { deployUpgradableFromFactory } from "../scripts/hardhat/utils";
import { tryVerifyContract, LogContractDeployment, getRequiredEnvVar } from "../common/helpers";
import { ROLLUP_REVENUE_VAULT_INITIALIZE_SIGNATURE } from "../common/constants";

const func: DeployFunction = async function () {
  const contractName = "RollupRevenueVault";

  const lastInvoiceDate = getRequiredEnvVar("ROLLUP_REVENUE_VAULT_LAST_INVOICE_DATE");
  const securityCouncil = getRequiredEnvVar("L2_SECURITY_COUNCIL");
  const invoiceSubmitter = getRequiredEnvVar("ROLLUP_REVENUE_VAULT_INVOICE_SUBMITTER");
  const burner = getRequiredEnvVar("ROLLUP_REVENUE_VAULT_BURNER");
  const invoicePaymentReceiver = getRequiredEnvVar("ROLLUP_REVENUE_VAULT_INVOICE_PAYMENT_RECEIVER");
  const tokenBridge = getRequiredEnvVar("ROLLUP_REVENUE_VAULT_TOKEN_BRIDGE");
  const messageService = getRequiredEnvVar("L2_MESSAGE_SERVICE_ADDRESS");
  const l1LineaTokenBurner = getRequiredEnvVar("ROLLUP_REVENUE_VAULT_L1_LINEA_TOKEN_BURNER");
  const lineaToken = getRequiredEnvVar("ROLLUP_REVENUE_VAULT_LINEA_TOKEN");
  const dexSwapAdapter = getRequiredEnvVar("ROLLUP_REVENUE_VAULT_DEX_SWAP_ADAPTER");

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
      unsafeAllow: ["constructor"],
    },
  );

  await LogContractDeployment(contractName, contract);
  const contractAddress = await contract.getAddress();

  await tryVerifyContract(contractAddress, "src/operational/RollupRevenueVault.sol:RollupRevenueVault");
};

export default func;
func.tags = ["RollupRevenueVault"];
