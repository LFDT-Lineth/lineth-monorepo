import { ethers, Provider } from "ethers";
import { readFileSync } from "fs";
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

type GenesisBasedAccountManagerOptions = {
  provider: Provider;
  genesisFilePath: string;
  excludeAddresses?: string[];
  priorityAddresses?: string[];
};

function readJsonFile(filePath: string): unknown {
  const data = readFileSync(filePath, "utf8");
  return JSON.parse(data);
}

function normalizeAddress(address: string): string {
  return ethers.getAddress(address);
}

function readGenesisFileAccounts(genesisJson: GenesisJson): Account[] {
  const alloc = genesisJson.alloc;
  const accounts: Account[] = [];
  for (const address in alloc) {
    const accountData = alloc[address];
    if (accountData.privateKey) {
      const addr = normalizeAddress(address);
      accounts.push(new Account(accountData.privateKey, addr));
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

function prioritizeAccounts(accounts: Account[], priorityAddresses: string[]): Account[] {
  if (!priorityAddresses || priorityAddresses.length === 0) {
    return accounts;
  }

  const normalizedPriorityAddresses = priorityAddresses.map(normalizeAddress);
  const priorityAccounts = accounts.filter((account) => normalizedPriorityAddresses.includes(account.address));
  const remainingAccounts = filterExcludedAccounts(
    accounts,
    priorityAccounts.map((account) => account.address),
  );
  return [...priorityAccounts, ...remainingAccounts];
}

class GenesisBasedAccountManager extends AccountManager {
  constructor(options: GenesisBasedAccountManagerOptions) {
    const { provider, genesisFilePath, excludeAddresses = [], priorityAddresses = [] } = options;

    const genesisJson = readJsonFile(genesisFilePath);
    const genesis = genesisJson as GenesisJson;
    const chainId = genesis.config.chainId;
    const accounts = readGenesisFileAccounts(genesis);
    const filteredAccounts = filterExcludedAccounts(accounts, excludeAddresses);
    const whaleAccounts = prioritizeAccounts(filteredAccounts, priorityAddresses);

    super(provider, whaleAccounts, chainId);
  }
}

export { GenesisBasedAccountManager };
