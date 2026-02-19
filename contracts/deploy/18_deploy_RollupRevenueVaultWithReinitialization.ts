import { DeployFunction } from "hardhat-deploy/types";
import { tryVerifyContract, getRequiredEnvVar } from "../common/helpers";
import { ethers, upgrades } from "hardhat";
import { RollupRevenueVault__factory } from "contracts/typechain-types";

const func: DeployFunction = async function () {
  const contractName = "RollupRevenueVault";

  const proxyAddress = getRequiredEnvVar("ROLLUP_REVENUE_VAULT_ADDRESS");
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

  const factory = await ethers.getContractFactory(contractName);

  console.log("Deploying Contract...");
  const newContract = await upgrades.deployImplementation(factory, {
    kind: "transparent",
  });

  const contractAddress = newContract.toString();

  console.log(`Contract deployed at ${contractAddress}`);

  const upgradeCallWithReinitializationUsingSecurityCouncil = ethers.concat([
    "0x9623609d",
    ethers.AbiCoder.defaultAbiCoder().encode(
      ["address", "address", "bytes"],
      [
        proxyAddress,
        newContract,
        RollupRevenueVault__factory.createInterface().encodeFunctionData("initializeRolesAndStorageVariables", [
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
        ]),
      ],
    ),
  ]);

  console.log(
    "Encoded Tx Upgrade with Reinitialization from Security Council:",
    "\n",
    upgradeCallWithReinitializationUsingSecurityCouncil,
  );
  console.log("\n");

  await tryVerifyContract(contractAddress, "src/operational/RollupRevenueVault.sol:RollupRevenueVault");
};

export default func;
func.tags = ["RollupRevenueVaultWithReinitialization"];
