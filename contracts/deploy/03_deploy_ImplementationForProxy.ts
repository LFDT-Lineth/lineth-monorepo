import { ethers, upgrades } from "hardhat";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";

import { getRequiredEnvVar, requireAddressFromRegistryOrEnv, tryVerifyContract } from "../common/helpers";
import { getUiSigner, withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";

/**
 * Maps every CONTRACT_NAME supported by this script to the registry key and env var used to
 * resolve its proxy address.
 *
 * - `registryKey` must match the key in `contracts/deployments/addresses/<network>.json`
 *   (see contracts/deployments/addresses/README.md for the full registry key reference).
 * - `envVar` is the canonical environment variable used as fallback when the network has no
 *   registry file or the entry is absent/zero.
 *
 * To add a new contract: add an entry here, add the key to the registry README table, and
 * populate the address in the relevant network JSON files.
 *
 * TokenBridge resolves to either TokenBridge_L1 or TokenBridge_L2 depending on
 * DEPLOY_TOKEN_BRIDGE_ON_L1.
 *
 * Validium and CallForwardingProxy have no canonical env var in the address registry,
 * so they use the generic PROXY_ADDRESS fallback.
 */
const CONTRACT_PROXY_MAP: Record<string, { registryKey: string; envVar: string }> = {
  LineaRollup: { registryKey: "LineaRollup", envVar: "LINEA_ROLLUP_ADDRESS" },
  Validium: { registryKey: "Validium", envVar: "PROXY_ADDRESS" },
  L2MessageService: { registryKey: "L2MessageService", envVar: "L2_MESSAGE_SERVICE_ADDRESS" },
  TokenBridge: {
    registryKey: process.env.DEPLOY_TOKEN_BRIDGE_ON_L1 === "true" ? "TokenBridge_L1" : "TokenBridge_L2",
    envVar: "TOKEN_BRIDGE_ADDRESS",
  },
  CallForwardingProxy: { registryKey: "CallForwardingProxy", envVar: "PROXY_ADDRESS" },
  YieldManager: { registryKey: "YieldManager", envVar: "YIELD_MANAGER_ADDRESS" },
  RollupRevenueVault: { registryKey: "RollupRevenueVault", envVar: "ROLLUP_REVENUE_VAULT_ADDRESS" },
};

const func: DeployFunction = withSignerUiSession(
  "03_deploy_ImplementationForProxy.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const signer = await getUiSigner(hre);
    const contractName = getRequiredEnvVar("CONTRACT_NAME");

    const proxyMapEntry = CONTRACT_PROXY_MAP[contractName];
    if (!proxyMapEntry) {
      throw new Error(
        `CONTRACT_NAME "${contractName}" is not supported by this script. ` +
          `Add an entry to CONTRACT_PROXY_MAP in 03_deploy_ImplementationForProxy.ts ` +
          `and update contracts/deployments/addresses/README.md.`,
      );
    }
    const { registryKey, envVar: proxyEnvVar } = proxyMapEntry;
    const proxyAddress = requireAddressFromRegistryOrEnv(hre.network.name, registryKey, proxyEnvVar);

    const factory = await ethers.getContractFactory(contractName, signer);

    console.log("Deploying Contract...");
    const newContract = await upgrades.deployImplementation(factory, {
      kind: "transparent",
    });

    const contract = newContract.toString();

    console.log(`Contract deployed at ${contract}`);

    const upgradeCallUsingSecurityCouncil = ethers.concat([
      "0x99a88ec4",
      ethers.AbiCoder.defaultAbiCoder().encode(["address", "address"], [proxyAddress, newContract]),
    ]);

    console.log("Encoded Tx Upgrade from Security Council:", "\n", upgradeCallUsingSecurityCouncil);

    console.log("\n");

    await tryVerifyContract(contract);
  },
);

export default func;
func.tags = ["ImplementationForProxy"];
