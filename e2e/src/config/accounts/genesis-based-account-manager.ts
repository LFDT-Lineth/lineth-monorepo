import { normalizeAddress } from "@consensys/linea-shared-utils";
import { readFileSync } from "fs";
import { Client } from "viem";

import Account from "./account";
import { AccountManager } from "./account-manager";

interface GenesisJson {
  config: {
    chainId: number;
  };
  alloc: {
    [address: string]: {
      privateKey?: string;
    };
  };
}

/**
 * @notice Options for creating a GenesisBasedAccountManager.
 * @property client The viem Client instance to interact with the chain.
 * @property genesisFilePath Path to the genesis file (JSON) containing accounts data.
 * @property excludeAddresses (Optional) List of addresses to be excluded from the managed accounts.
 * @property reservedAddresses (Optional) List of addresses to be prioritized or reserved among managed accounts.
 */
type GenesisBasedAccountManagerOptions = {
  client: Client;
  genesisFilePath: string;
  excludeAddresses?: string[];
  reservedAddresses?: string[];
};

function readJsonFile(filePath: string): unknown {
  const data = readFileSync(filePath, "utf8");
  return JSON.parse(data);
}

function readGenesisFileAccounts(genesisJson: GenesisJson): Account[] {
  const alloc = genesisJson.alloc;
  const accounts: Account[] = [];
  for (const address in alloc) {
    const accountData = alloc[address];
    if (accountData.privateKey) {
      const prefixedAddress = address.startsWith("0x") ? address : `0x${address}`;
      const addr = normalizeAddress(prefixedAddress);
      accounts.push(new Account(accountData.privateKey as `0x${string}`, addr));
    }
  }
  return accounts;
}

function filterExcludedAccounts(accounts: Account[], excludeAddresses: string[]): Account[] {
  if (!excludeAddresses || excludeAddresses.length === 0) {
    return accounts;
  }

  const normalizedExcludeAddresses = excludeAddresses.map(normalizeAddress);
  return accounts.filter((account) => !normalizedExcludeAddresses.includes(account.address));
}

function prioritizeReservedAccounts(accounts: Account[], reservedAddresses: string[]): Account[] {
  if (!reservedAddresses || reservedAddresses.length === 0) {
    return accounts;
  }

  const normalizedReservedAddresses = reservedAddresses.map(normalizeAddress);
  const accountsByAddress = new Map(accounts.map((account) => [account.address, account]));
  const reservedAccounts = normalizedReservedAddresses
    .map((address) => accountsByAddress.get(address))
    .filter((account): account is Account => Boolean(account));
  const remainingAccounts = filterExcludedAccounts(
    accounts,
    reservedAccounts.map((account) => account.address),
  );

  return [...reservedAccounts, ...remainingAccounts];
}

class GenesisBasedAccountManager extends AccountManager {
  constructor(options: GenesisBasedAccountManagerOptions) {
    const { client, genesisFilePath, excludeAddresses = [], reservedAddresses = [] } = options;

    const genesisJson = readJsonFile(genesisFilePath);
    const genesis = genesisJson as GenesisJson;
    const chainId = genesis.config.chainId;
    const accounts = readGenesisFileAccounts(genesis);
    const filteredAccounts = filterExcludedAccounts(accounts, excludeAddresses);
    const whaleAccounts = prioritizeReservedAccounts(filteredAccounts, reservedAddresses);

    super(client, whaleAccounts, chainId, reservedAddresses);
  }
}

export { GenesisBasedAccountManager };
