import { PRECOMPILES_ADDRESSES } from "contracts/common/constants";
import { ethers } from "hardhat";
import { HardhatRuntimeEnvironment } from "hardhat/types";
import { DeployFunction } from "hardhat-deploy/types";
import { readFileSync } from "node:fs";
import { join } from "node:path";

import {
  LogContractDeployment,
  getOptionalEnvVar,
  requireAddressFromRegistryOrEnv,
  tryVerifyContractWithConstructorArgs,
} from "../common/helpers";
import { formatEnvVarForLog } from "../common/helpers/envVarLogging";
import { getUiSigner, withSignerUiSession } from "../scripts/hardhat/signer-ui-bridge";

const DEFAULT_FILTERED_ADDRESS_PATH = join(__dirname, "..", "addresses-filter.txt");

/** Keeps each `setFilteredStatus` tx under Osaka-era gas limits (cold mapping writes per address). */
const SET_FILTERED_STATUS_BATCH_SIZE = 300;

/**
 * Reads filtered addresses from the file at `ADDRESS_FILTER_FILE_PATH` (env) or
 * `contracts/addresses-filter.txt` (default). YAML-style list entries; excludes
 * precompiles already covered by {@link PRECOMPILES_ADDRESSES}.
 *
 * Called lazily inside the deploy function so a missing file does not crash unrelated
 * Hardhat deploy runs that load this module for other `--tags`.
 */
export function loadAddressFilterFilteredAddressesFromFile(): string[] {
  const customPath = getOptionalEnvVar("ADDRESS_FILTER_FILE_PATH");
  const filePath = customPath ?? DEFAULT_FILTERED_ADDRESS_PATH;
  const precompileSet = new Set(PRECOMPILES_ADDRESSES.map((a) => a.toLowerCase()));
  const text = readFileSync(filePath, "utf8");
  if (customPath) {
    console.log(
      `AddressFilter: loading filtered addresses from ${formatEnvVarForLog("ADDRESS_FILTER_FILE_PATH", customPath)}`,
    );
  } else {
    console.log("AddressFilter: loading filtered addresses from default addresses-filter.txt");
  }
  const entryRe = /-\s*"(0x[a-fA-F0-9]{40})"/g;
  const seen = new Set<string>();
  const out: string[] = [];
  let m: RegExpExecArray | null;
  while ((m = entryRe.exec(text)) !== null) {
    const addr = `0x${m[1].slice(2).toLowerCase()}`;
    const key = addr.toLowerCase();
    if (precompileSet.has(key) || seen.has(key)) {
      continue;
    }
    seen.add(key);
    out.push(addr);
  }
  return out;
}

const func: DeployFunction = withSignerUiSession(
  "21_deploy_AddressFilter.ts",
  async function (hre: HardhatRuntimeEnvironment) {
    const contractName = "AddressFilter";
    const signer = await getUiSigner(hre);

    const councilAddress = requireAddressFromRegistryOrEnv(
      hre.network.name,
      "L1_SECURITY_COUNCIL",
      "L1_SECURITY_COUNCIL",
    );
    const deployerAddress = ethers.getAddress(await signer.getAddress());
    const filteredAddresses = loadAddressFilterFilteredAddressesFromFile();

    // Constructor initcode must stay under EIP-3860 (49152 bytes). Deployer is temporary DEFAULT_ADMIN.
    const constructorInitialList = [...PRECOMPILES_ADDRESSES];

    const factory = await ethers.getContractFactory(contractName, signer);
    const contract = await factory.deploy(deployerAddress, constructorInitialList);

    await LogContractDeployment(contractName, contract);
    const contractAddress = await contract.getAddress();

    if (filteredAddresses.length > 0) {
      const batchTotal = Math.ceil(filteredAddresses.length / SET_FILTERED_STATUS_BATCH_SIZE);
      for (let i = 0; i < filteredAddresses.length; i += SET_FILTERED_STATUS_BATCH_SIZE) {
        const chunk = filteredAddresses.slice(i, i + SET_FILTERED_STATUS_BATCH_SIZE);
        const batchIndex = Math.floor(i / SET_FILTERED_STATUS_BATCH_SIZE) + 1;
        console.log(`AddressFilter: setFilteredStatus batch ${batchIndex}/${batchTotal} (${chunk.length} addresses)…`);
        const tx = await contract.setFilteredStatus(chunk, true);
        await tx.wait();
      }
    }

    const defaultAdminRole = await contract.DEFAULT_ADMIN_ROLE();

    if (deployerAddress !== councilAddress) {
      console.log(`AddressFilter: granting DEFAULT_ADMIN_ROLE to L1_SECURITY_COUNCIL ${councilAddress}…`);
      await (await contract.grantRole(defaultAdminRole, councilAddress)).wait();
      console.log("AddressFilter: deployer renouncing DEFAULT_ADMIN_ROLE…");
      await (await contract.renounceRole(defaultAdminRole, deployerAddress)).wait();
    } else {
      console.log("AddressFilter: deployer equals L1_SECURITY_COUNCIL — skipping grant/renounce (admin unchanged).");
    }

    const args = [deployerAddress, constructorInitialList];
    await tryVerifyContractWithConstructorArgs(
      contractAddress,
      "src/rollup/forcedTransactions/AddressFilter.sol:AddressFilter",
      args,
    );
  },
);

export default func;
func.tags = ["AddressFilter"];
