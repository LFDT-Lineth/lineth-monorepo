import { RollupRevenueVault__factory } from "contracts/typechain-types";
import { ethers, upgrades } from "hardhat";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import { tryVerifyContract, getRequiredEnvVar, requireAddressFromRegistryOrEnv } from "../common/helpers";
import { getUiSigner, withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";

const func: DeployFunction = withSignerUiSession(
  "18_deploy_RollupRevenueVaultWithReinitialization.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const signer = await getUiSigner(hre);
    const contractName = "RollupRevenueVault";

    const proxyAddress = requireAddressFromRegistryOrEnv(
      hre.network.name,
      "RollupRevenueVault",
      "ROLLUP_REVENUE_VAULT_ADDRESS",
    );
    const lastInvoiceDate = getRequiredEnvVar("ROLLUP_REVENUE_VAULT_LAST_INVOICE_DATE");
    const securityCouncil = requireAddressFromRegistryOrEnv(
      hre.network.name,
      "L2_SECURITY_COUNCIL",
      "L2_SECURITY_COUNCIL",
    );
    const invoiceSubmitter = requireAddressFromRegistryOrEnv(
      hre.network.name,
      "ROLLUP_REVENUE_VAULT_INVOICE_SUBMITTER",
      "ROLLUP_REVENUE_VAULT_INVOICE_SUBMITTER",
    );
    const burner = requireAddressFromRegistryOrEnv(
      hre.network.name,
      "ROLLUP_REVENUE_VAULT_BURNER",
      "ROLLUP_REVENUE_VAULT_BURNER",
    );
    const invoicePaymentReceiver = requireAddressFromRegistryOrEnv(
      hre.network.name,
      "ROLLUP_REVENUE_VAULT_INVOICE_PAYMENT_RECEIVER",
      "ROLLUP_REVENUE_VAULT_INVOICE_PAYMENT_RECEIVER",
    );
    const tokenBridge = requireAddressFromRegistryOrEnv(hre.network.name, "TokenBridge_L2", "TOKEN_BRIDGE_ADDRESS");
    const messageService = requireAddressFromRegistryOrEnv(
      hre.network.name,
      "L2MessageService",
      "L2_MESSAGE_SERVICE_ADDRESS",
    );
    const l1LineaTokenBurner = requireAddressFromRegistryOrEnv(
      hre.network.name,
      "ROLLUP_REVENUE_VAULT_L1_LINEA_TOKEN_BURNER",
      "ROLLUP_REVENUE_VAULT_L1_LINEA_TOKEN_BURNER",
    );
    const lineaToken = requireAddressFromRegistryOrEnv(
      hre.network.name,
      "ROLLUP_REVENUE_VAULT_LINEA_TOKEN",
      "ROLLUP_REVENUE_VAULT_LINEA_TOKEN",
    );
    const dexSwapAdapter = requireAddressFromRegistryOrEnv(
      hre.network.name,
      "ROLLUP_REVENUE_VAULT_DEX_SWAP_ADAPTER",
      "ROLLUP_REVENUE_VAULT_DEX_SWAP_ADAPTER",
    );

    const factory = await ethers.getContractFactory(contractName, signer);

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
  },
);

export default func;
func.tags = ["RollupRevenueVaultWithReinitialization"];
