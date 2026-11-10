import { readFileSync } from "node:fs";
import { join } from "node:path";
import { type Address, getAddress } from "viem";

import { PRECOMPILES_ADDRESSES } from "../../../common/constants/index.js";
import { getOptionalEnvVar } from "../../../common/helpers/environment.js";
import { formatEnvVarForLog } from "../../../common/helpers/envVarLogging.js";
import { requireAddressFromRegistryOrEnv } from "../../../common/helpers/readAddress.js";
import { getUiWalletClient } from "../../../hardhat-plugins/signer-ui/session.js";

import type { DeployCompanion, ParameterContext } from "../../../hardhat-plugins/deploy/types.js";
import type { WalletClient } from "@nomicfoundation/hardhat-viem/types";
import type { ModuleParameters } from "@nomicfoundation/ignition-core";
import type { NetworkConnection } from "hardhat/types/network";

/** `21_deploy_AddressFilter.ts`: the module has no parameters; resolve the Security Council now so a gap fails early. */
export function addressFilterPreflight(context: ParameterContext): ModuleParameters {
  requireAddressFromRegistryOrEnv(context.networkName, "L1_SECURITY_COUNCIL", "L1_SECURITY_COUNCIL");
  return {};
}

/** The module has no parameters; resolve the Security Council now so a gap fails early. */
export const parameters: NonNullable<DeployCompanion["parameters"]> = async (context) => {
  addressFilterPreflight(context);
  return {};
};

const DEFAULT_FILTERED_ADDRESS_PATH = join(import.meta.dirname, "..", "..", "..", "addresses-filter.txt");

/** Keeps each `setFilteredStatus` tx under Osaka-era gas limits (cold mapping writes per address). */
export const SET_FILTERED_STATUS_BATCH_SIZE = 300;

/**
 * Reads filtered addresses from the file at `ADDRESS_FILTER_FILE_PATH` (env) or
 * `contracts/addresses-filter.txt` (default). YAML-style list entries; excludes
 * precompiles already covered by {@link PRECOMPILES_ADDRESSES}.
 */
export function loadAddressFilterFilteredAddressesFromFile(): Address[] {
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
  const out: Address[] = [];
  let m: RegExpExecArray | null;
  while ((m = entryRe.exec(text)) !== null) {
    const matched = m[1];
    if (matched === undefined) {
      continue;
    }
    const key = matched.toLowerCase();
    if (precompileSet.has(key) || seen.has(key)) {
      continue;
    }
    seen.add(key);
    out.push(getAddress(key));
  }
  return out;
}

/**
 * Second half of `21_deploy_AddressFilter.ts`: `setFilteredStatus` in batches, then hand `DEFAULT_ADMIN_ROLE` to
 * `L1_SECURITY_COUNCIL` and renounce it (skipped when the deployer is the council). Safe to re-run: stops early when the
 * signer no longer holds `DEFAULT_ADMIN_ROLE` (configuration already done).
 */
export async function configureAddressFilter(
  connection: NetworkConnection,
  addressFilterAddress: Address,
  wallet: WalletClient,
): Promise<void> {
  const councilAddress = getAddress(
    requireAddressFromRegistryOrEnv(connection.networkName, "L1_SECURITY_COUNCIL", "L1_SECURITY_COUNCIL"),
  );
  const deployerAddress = getAddress(wallet.account.address);
  const publicClient = await connection.viem.getPublicClient();
  const contract = await connection.viem.getContractAt("AddressFilter", addressFilterAddress, {
    client: { public: publicClient, wallet },
  });

  const defaultAdminRole = await contract.read.DEFAULT_ADMIN_ROLE();
  if (!(await contract.read.hasRole([defaultAdminRole, deployerAddress]))) {
    console.log(
      `AddressFilter: ${deployerAddress} does not hold DEFAULT_ADMIN_ROLE on ${addressFilterAddress}; configuration already done.`,
    );
    return;
  }

  const filteredAddresses = loadAddressFilterFilteredAddressesFromFile();
  if (filteredAddresses.length > 0) {
    const batchTotal = Math.ceil(filteredAddresses.length / SET_FILTERED_STATUS_BATCH_SIZE);
    for (let i = 0; i < filteredAddresses.length; i += SET_FILTERED_STATUS_BATCH_SIZE) {
      const chunk = filteredAddresses.slice(i, i + SET_FILTERED_STATUS_BATCH_SIZE);
      const batchIndex = Math.floor(i / SET_FILTERED_STATUS_BATCH_SIZE) + 1;
      console.log(`AddressFilter: setFilteredStatus batch ${batchIndex}/${batchTotal} (${chunk.length} addresses)…`);
      const hash = await contract.write.setFilteredStatus([chunk, true], { account: wallet.account });
      await publicClient.waitForTransactionReceipt({ hash });
    }
  }

  if (deployerAddress !== councilAddress) {
    console.log(`AddressFilter: granting DEFAULT_ADMIN_ROLE to L1_SECURITY_COUNCIL ${councilAddress}…`);
    await publicClient.waitForTransactionReceipt({
      hash: await contract.write.grantRole([defaultAdminRole, councilAddress], { account: wallet.account }),
    });
    console.log("AddressFilter: deployer renouncing DEFAULT_ADMIN_ROLE…");
    await publicClient.waitForTransactionReceipt({
      hash: await contract.write.renounceRole([defaultAdminRole, deployerAddress], { account: wallet.account }),
    });
  } else {
    console.log("AddressFilter: deployer equals L1_SECURITY_COUNCIL — skipping grant/renounce (admin unchanged).");
  }
}

/** Filters the configured addresses and hands `DEFAULT_ADMIN_ROLE` to the Security Council (idempotent). */
export const afterDeploy: NonNullable<DeployCompanion["afterDeploy"]> = async ({ connection, contracts }) => {
  const addressFilter = contracts["AddressFilter#AddressFilter"];
  if (addressFilter === undefined) {
    throw new Error('Future "AddressFilter#AddressFilter" is not in the deployment result.');
  }
  await configureAddressFilter(connection, addressFilter, await getUiWalletClient(connection));
};
