import { Client } from "viem";

import Account from "./account";
import { AccountManager } from "./account-manager";

class EnvironmentBasedAccountManager extends AccountManager {
  constructor(client: Client, whaleAccounts: Account[], chainId: number) {
    super(client, whaleAccounts, chainId);
  }
}

export { EnvironmentBasedAccountManager };
